// src/test/scala/UnitTests.scala
import org.scalatest.flatspec.AnyFlatSpec
import org.scalatest.matchers.should.Matchers
import Calculator._

class UnitTests extends AnyFlatSpec with Matchers {

  "addAmount" should "correctly add amount to an address" in {
    val initialAccounts = Map("address1" -> 100.0, "address2" -> 50.0)
    val updatedAccounts = Calculator.addAmount("address1", 50.0, initialAccounts)
    updatedAccounts.getOrElse("address1", 0.0) shouldEqual 150.0
  }

  it should "create a new entry if address does not exist" in {
    val initialAccounts = Map("address1" -> 100.0, "address2" -> 50.0)
    val updatedAccounts = Calculator.addAmount("address3", 75.0, initialAccounts)
    updatedAccounts.getOrElse("address3", 0.0) shouldEqual 75.0
  }

  "subtractAmount" should "correctly subtract amount from an address" in {
    val initialAccounts = Map("address1" -> 100.0, "address2" -> 50.0)
    val result = Calculator.subtractAmount("address1", 50.0, initialAccounts)
    result.getOrElse("address1", 0.0) shouldEqual 50.0
  }

  it should "return a Left if amount to subtract exceeds available balance" in {
    val initialAccounts = Map("address1" -> 100.0, "address2" -> 50.0)
    val result = Calculator.subtractAmount("address2", 60.0, initialAccounts)
    result shouldEqual Left(TransactionError("Insufficient funds: Cannot subtract amount greater than available balance"))
  }

  it should "return a Left if address does not exist" in {
    val initialAccounts = Map("address1" -> 100.0, "address2" -> 50.0)
    val result = Calculator.subtractAmount("address3", 10.0, initialAccounts)
    result shouldEqual Left(TransactionError("Address not found: Cannot subtract amount from non-existing account"))
  }
}
