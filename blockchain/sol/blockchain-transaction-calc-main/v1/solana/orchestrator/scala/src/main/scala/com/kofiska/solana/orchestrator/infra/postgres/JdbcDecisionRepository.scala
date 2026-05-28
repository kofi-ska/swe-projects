package com.kofiska.solana.orchestrator.infra.postgres

import com.kofiska.solana.orchestrator.domain.{Actionability, DecisionResult, RequestContext, TerminalState}
import com.kofiska.solana.orchestrator.ports.DecisionRepository

import java.sql.{Connection, DriverManager, ResultSet}
import scala.concurrent.{ExecutionContext, Future, blocking}
import scala.util.Using

final class JdbcDecisionRepository(
  jdbcUrl: String,
  user: String,
  password: String
)(implicit ec: ExecutionContext) extends DecisionRepository {

  private def connection: Connection =
    DriverManager.getConnection(jdbcUrl, user, password)

  override def find(requestId: String): Future[Option[DecisionResult]] =
    Future(blocking {
      Using.resource(connection) { conn =>
        val statement = conn.prepareStatement(
          """SELECT request_id, decision_id, terminal_state, reason_code, actionability, best_route_id,
            |expected_output, fee_cost, slippage_cost, breakeven_margin, ev_estimate, ev_lower_bound,
            |risk_score, freshness_valid
            |FROM decisions
            |WHERE request_id = ?""".stripMargin
        )
        statement.setString(1, requestId)
        Using.resource(statement) { ps =>
          val rows = ps.executeQuery()
          if (rows.next()) Some(rowToDecision(rows)) else None
        }
      }
    })

  override def upsert(ctx: RequestContext, result: DecisionResult): Future[Unit] =
    Future(blocking {
      Using.resource(connection) { conn =>
        val statement = conn.prepareStatement(
          """INSERT INTO decisions (
            |request_id, dedupe_key, decision_id, terminal_state, reason_code, actionability,
            |model_version, route_id, slot, quote_age, source_hashes, expected_output, fee_cost,
            |slippage_cost, breakeven_margin, ev_estimate, ev_lower_bound, risk_score, freshness_valid
            |) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            |ON CONFLICT (request_id) DO UPDATE SET
            |dedupe_key = EXCLUDED.dedupe_key,
            |decision_id = EXCLUDED.decision_id,
            |terminal_state = EXCLUDED.terminal_state,
            |reason_code = EXCLUDED.reason_code,
            |actionability = EXCLUDED.actionability,
            |model_version = EXCLUDED.model_version,
            |route_id = EXCLUDED.route_id,
            |slot = EXCLUDED.slot,
            |quote_age = EXCLUDED.quote_age,
            |source_hashes = EXCLUDED.source_hashes,
            |expected_output = EXCLUDED.expected_output,
            |fee_cost = EXCLUDED.fee_cost,
            |slippage_cost = EXCLUDED.slippage_cost,
            |breakeven_margin = EXCLUDED.breakeven_margin,
            |ev_estimate = EXCLUDED.ev_estimate,
            |ev_lower_bound = EXCLUDED.ev_lower_bound,
            |risk_score = EXCLUDED.risk_score,
            |freshness_valid = EXCLUDED.freshness_valid""".stripMargin
        )
        Using.resource(statement) { ps =>
          ps.setString(1, ctx.requestId)
          ps.setString(2, ctx.dedupeKey)
          ps.setString(3, result.decisionId)
          ps.setString(4, TerminalState.asString(result.terminalState))
          ps.setString(5, result.reasonCode)
          ps.setString(6, Actionability.asString(result.actionability))
          ps.setString(7, ctx.modelVersion)
          ps.setString(8, result.bestRouteId.orNull)
          ps.setLong(9, ctx.slot)
          ps.setLong(10, ctx.quoteAge)
          ps.setString(11, ctx.sourceHashes.mkString(","))
          ps.setString(12, result.expectedOutput.orNull)
          ps.setString(13, result.feeCost.orNull)
          ps.setString(14, result.slippageCost.orNull)
          ps.setString(15, result.breakevenMargin.orNull)
          ps.setString(16, result.evEstimate.orNull)
          ps.setString(17, result.evLowerBound.orNull)
          ps.setString(18, result.riskScore.orNull)
          ps.setBoolean(19, result.freshnessValid)
          ps.executeUpdate()
          ()
        }
      }
    })

  private def rowToDecision(row: ResultSet): DecisionResult =
    DecisionResult(
      requestId = row.getString("request_id"),
      decisionId = row.getString("decision_id"),
      terminalState = TerminalState.fromString(row.getString("terminal_state")),
      reasonCode = row.getString("reason_code"),
      actionability = Actionability.fromString(row.getString("actionability")),
      bestRouteId = Option(row.getString("best_route_id")).filter(_.nonEmpty),
      expectedOutput = Option(row.getString("expected_output")).filter(_.nonEmpty),
      feeCost = Option(row.getString("fee_cost")).filter(_.nonEmpty),
      slippageCost = Option(row.getString("slippage_cost")).filter(_.nonEmpty),
      breakevenMargin = Option(row.getString("breakeven_margin")).filter(_.nonEmpty),
      evEstimate = Option(row.getString("ev_estimate")).filter(_.nonEmpty),
      evLowerBound = Option(row.getString("ev_lower_bound")).filter(_.nonEmpty),
      riskScore = Option(row.getString("risk_score")).filter(_.nonEmpty),
      freshnessValid = row.getBoolean("freshness_valid")
    )
}
