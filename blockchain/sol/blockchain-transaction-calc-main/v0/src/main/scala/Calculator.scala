// src/main/scala/Calculator.scala

object Calculator {

  case class TransactionError(message: String) extends Exception(message)

  // Function to add an amount to an address
  def addAmount(address: String, amount: Double, accounts: Map[String, Double]): Map[String, Double] = {
    accounts.updatedWith(address) {
      case Some(existingAmount) => Some(existingAmount + amount)
      case None => Some(amount)
    }
  }

  // Function to subtract an amount from an address
  def subtractAmount(address: String, amount: Double, accounts: Map[String, Double]): Either[TransactionError, Map[String, Double]] = {
    accounts.get(address) match {
      case Some(existingAmount) =>
        val newAmount = existingAmount - amount
        if (newAmount < 0)
          Left(TransactionError("Insufficient funds: Cannot subtract amount greater than available balance"))
        else
          Right(accounts.updated(address, newAmount))
      case None =>
        Left(TransactionError("Address not found: Cannot subtract amount from non-existing account"))
    }
  }
}
