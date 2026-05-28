import org.scalatest.flatspec.AnyFlatSpec
import org.scalatest.matchers.should.Matchers
import scala.concurrent.{Await, ExecutionContext, Future}
import scala.concurrent.duration.Duration
import scala.util.Random
import Calculator._

class IntegrationTest extends AnyFlatSpec with Matchers {

  implicit val executionContext: ExecutionContext = ExecutionContext.global

  "Blockchain Transaction Calculator" should "correctly perform random computations" in {
    val numComputations = 12000
    val computationsPerThread = numComputations / 6
    var accounts = Map("address1" -> 100.0, "address2" -> 50.0)

    // Create a list of Future computations to be executed
    val futures = (1 to 6).map { _ =>
      Future {
        (1 to computationsPerThread).foreach { _ =>
          val address = RandomDataGenerator.generateRandomAddress()
          val amount = RandomDataGenerator.generateRandomAmount()

          // Perform random computation (add or subtract)
          val operation = if (Random.nextBoolean()) "add" else "subtract"
          operation match {
            case "add" =>
              accounts = addAmount(address, amount, accounts)
              println(s"Successfully performed $operation operation on $address")
            case "subtract" =>
              subtractAmount(address, amount, accounts) match {
                case Right(updatedAccounts) =>
                  accounts = updatedAccounts
                  println(s"Successfully performed $operation operation on $address")
                case Left(error) =>
                  println(s"Failed to perform $operation operation on $address: ${error.message}")
              }
          }
        }
      }
    }

    // Combine all futures into a single future representing completion of all computations
    val allComputations = Future.sequence(futures)

    // Wait for all computations to complete
    Await.result(allComputations, Duration.Inf)

    // Assertions to verify final state of accounts
    accounts.values.foreach { amount =>
      withClue(s"Amount $amount in accounts should be non-negative double: ") {
        amount should be >= 0.0
        amount shouldBe a[Double]
      }
    }
  }
}
