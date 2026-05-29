package com.kofiska.solana.orchestrator.infra.postgres

import com.kofiska.solana.orchestrator.domain.{Actionability, DecisionResult, RequestContext, TerminalState, TransitionEvent}
import com.kofiska.solana.orchestrator.ports.DecisionRepository
import com.zaxxer.hikari.{HikariConfig, HikariDataSource}

import java.sql.{Connection, ResultSet}
import scala.concurrent.{ExecutionContext, Future, blocking}
import scala.util.Using

final class JdbcDecisionRepository(
  jdbcUrl: String,
  user: String,
  password: String,
  poolSize: Int,
  connectionTimeoutMs: Long
)(implicit ec: ExecutionContext) extends DecisionRepository {

  private val dataSource = {
    val config = new HikariConfig()
    config.setJdbcUrl(jdbcUrl)
    config.setUsername(user)
    config.setPassword(password)
    config.setMaximumPoolSize(poolSize)
    config.setConnectionTimeout(connectionTimeoutMs)
    config.setPoolName("solana-decision-store")
    new HikariDataSource(config)
  }

  private def connection: Connection =
    dataSource.getConnection

  override def find(requestId: String): Future[Option[DecisionResult]] =
    Future(blocking {
      Using.resource(connection) { conn =>
        val statement = conn.prepareStatement(
          """SELECT request_id, decision_id, terminal_state, reason_code, actionability, route_id,
            |expected_output, fee_cost, slippage_cost, breakeven_margin, ev_estimate, ev_lower_bound,
            |risk_score, freshness_valid, source_hashes
            |FROM decisions
            |WHERE request_id = ?""".stripMargin
        )
        Using.resource(statement) { ps =>
          ps.setQueryTimeout(5)
          ps.setString(1, requestId)
          val rows = ps.executeQuery()
          if (rows.next()) Some(rowToDecision(rows)) else None
        }
      }
    })

  override def upsert(ctx: RequestContext, result: DecisionResult, event: TransitionEvent): Future[Unit] =
    Future(blocking {
      Using.resource(connection) { conn =>
        conn.setAutoCommit(false)
        try {
          val decisionStatement = conn.prepareStatement(
            """INSERT INTO decisions (
              |request_id, dedupe_key, decision_id, schema_version, terminal_state, reason_code, actionability,
              |model_version, route_id, slot, quote_age, source_hashes, expected_output, fee_cost,
              |slippage_cost, breakeven_margin, ev_estimate, ev_lower_bound, risk_score, freshness_valid
              |) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
              |ON CONFLICT (request_id) DO NOTHING""".stripMargin
          )
          Using.resource(decisionStatement) { ps =>
            ps.setQueryTimeout(5)
            ps.setString(1, ctx.requestId)
            ps.setString(2, ctx.dedupeKey)
            ps.setString(3, result.decisionId)
            ps.setString(4, "v1")
            ps.setString(5, TerminalState.asString(result.terminalState))
            ps.setString(6, result.reasonCode)
            ps.setString(7, Actionability.asString(result.actionability))
            ps.setString(8, ctx.modelVersion)
            ps.setString(9, result.bestRouteId.orNull)
            ps.setLong(10, ctx.slot)
            ps.setLong(11, ctx.quoteAge)
            ps.setArray(12, conn.createArrayOf("text", ctx.sourceHashes.map(_.asInstanceOf[AnyRef]).toArray))
            setBigDecimal(ps, 13, result.expectedOutput)
            setBigDecimal(ps, 14, result.feeCost)
            setBigDecimal(ps, 15, result.slippageCost)
            setBigDecimal(ps, 16, result.breakevenMargin)
            setBigDecimal(ps, 17, result.evEstimate)
            setBigDecimal(ps, 18, result.evLowerBound)
            setBigDecimal(ps, 19, result.riskScore)
            ps.setBoolean(20, result.freshnessValid)
            ps.executeUpdate()
          }

          val outboxStatement = conn.prepareStatement(
            """INSERT INTO audit_outbox (
              |request_id, decision_id, schema_version, trace_id, terminal_state, reason_code, model_version,
              |route_id, slot, quote_age, source_hashes, stage, latency_ms, bytes_in, bytes_out, success
              |) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
              |ON CONFLICT (request_id, decision_id) DO NOTHING""".stripMargin
          )
          Using.resource(outboxStatement) { ps =>
            ps.setQueryTimeout(5)
            ps.setString(1, event.requestId)
            ps.setString(2, event.decisionId)
            ps.setString(3, event.schemaVersion)
            ps.setString(4, event.traceId)
            ps.setString(5, event.terminalState)
            ps.setString(6, event.reasonCode)
            ps.setString(7, event.modelVersion)
            ps.setString(8, event.routeId.orNull)
            ps.setLong(9, event.slot)
            ps.setLong(10, event.quoteAge)
            ps.setArray(11, conn.createArrayOf("text", event.sourceHashes.map(_.asInstanceOf[AnyRef]).toArray))
            ps.setString(12, event.stage)
            ps.setLong(13, event.latencyMs)
            ps.setLong(14, event.bytesIn)
            ps.setLong(15, event.bytesOut)
            ps.setBoolean(16, event.success)
            ps.executeUpdate()
          }

          conn.commit()
          ()
        } catch {
          case error: Throwable =>
            conn.rollback()
            throw error
        } finally {
          conn.setAutoCommit(true)
        }
      }
    })

  override def pendingAudit(limit: Int): Future[Vector[TransitionEvent]] =
    Future(blocking {
      Using.resource(connection) { conn =>
        val statement = conn.prepareStatement(
          """SELECT request_id, decision_id, schema_version, trace_id, terminal_state, reason_code, model_version,
            |route_id, slot, quote_age, source_hashes, stage, latency_ms, bytes_in, bytes_out, success
            |FROM audit_outbox
            |WHERE published_at IS NULL
            |ORDER BY created_at ASC
            |LIMIT ?""".stripMargin
        )
        Using.resource(statement) { ps =>
          ps.setQueryTimeout(5)
          ps.setInt(1, limit)
          val rows = ps.executeQuery()
          val buffer = Vector.newBuilder[TransitionEvent]
          while (rows.next()) {
            buffer += outboxRow(rows)
          }
          buffer.result()
        }
      }
    })

  override def markAuditPublished(requestId: String, decisionId: String): Future[Unit] =
    Future(blocking {
      Using.resource(connection) { conn =>
        val statement = conn.prepareStatement(
          """UPDATE audit_outbox
            |SET published_at = NOW()
            |WHERE request_id = ? AND decision_id = ?""".stripMargin
        )
        Using.resource(statement) { ps =>
          ps.setQueryTimeout(5)
          ps.setString(1, requestId)
          ps.setString(2, decisionId)
          ps.executeUpdate()
          ()
        }
      }
    })

  override def close(): Unit =
    dataSource.close()

  private def rowToDecision(row: ResultSet): DecisionResult =
    DecisionResult(
      requestId = row.getString("request_id"),
      decisionId = row.getString("decision_id"),
      terminalState = TerminalState.fromString(row.getString("terminal_state")),
      reasonCode = row.getString("reason_code"),
      actionability = Actionability.fromString(row.getString("actionability")),
      bestRouteId = Option(row.getString("route_id")).filter(_.nonEmpty),
      sourceHashes = getStringVector(row, "source_hashes"),
      expectedOutput = getBigDecimal(row, "expected_output"),
      feeCost = getBigDecimal(row, "fee_cost"),
      slippageCost = getBigDecimal(row, "slippage_cost"),
      breakevenMargin = getBigDecimal(row, "breakeven_margin"),
      evEstimate = getBigDecimal(row, "ev_estimate"),
      evLowerBound = getBigDecimal(row, "ev_lower_bound"),
      riskScore = getBigDecimal(row, "risk_score"),
      freshnessValid = row.getBoolean("freshness_valid")
    )

  private def setBigDecimal(statement: java.sql.PreparedStatement, index: Int, value: Option[BigDecimal]): Unit =
    value match {
      case Some(decimal) => statement.setBigDecimal(index, decimal.bigDecimal)
      case None          => statement.setNull(index, java.sql.Types.NUMERIC)
    }

  private def getBigDecimal(row: ResultSet, column: String): Option[BigDecimal] = {
    val value = row.getBigDecimal(column)
    Option(value).map(BigDecimal(_))
  }

  private def getStringVector(row: ResultSet, column: String): Vector[String] =
    Option(row.getArray(column)).map { array =>
      array.getArray match {
        case values: Array[AnyRef] => values.collect { case value: String => value }.toVector
        case values: Array[String]  => values.toVector
        case _                      => Vector.empty[String]
      }
    }.getOrElse(Vector.empty)

  private def outboxRow(row: ResultSet): TransitionEvent =
    TransitionEvent(
      schemaVersion = row.getString("schema_version"),
      traceId = row.getString("trace_id"),
      requestId = row.getString("request_id"),
      decisionId = row.getString("decision_id"),
      terminalState = row.getString("terminal_state"),
      reasonCode = row.getString("reason_code"),
      modelVersion = row.getString("model_version"),
      routeId = Option(row.getString("route_id")).filter(_.nonEmpty),
      slot = row.getLong("slot"),
      quoteAge = row.getLong("quote_age"),
      sourceHashes = getStringVector(row, "source_hashes"),
      stage = row.getString("stage"),
      latencyMs = row.getLong("latency_ms"),
      bytesIn = row.getLong("bytes_in"),
      bytesOut = row.getLong("bytes_out"),
      success = row.getBoolean("success")
    )
}
