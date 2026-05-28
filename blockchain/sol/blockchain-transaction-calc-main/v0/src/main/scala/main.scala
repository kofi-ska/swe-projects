// src/main/scala/Main.scala
object Main {
  def main(args: Array[String]): Unit = {
    // Example usage
    //Addition to acc
    val initialAccounts = Map("address1" -> 100.0, "address2" -> 50.0)
    val updatedAccounts = Calculator.addAmount("address1", 50.0, initialAccounts)
    println(updatedAccounts)

    try {
      val finalAccounts = Calculator.subtractAmount("address2", 60.0, updatedAccounts)
      println(finalAccounts)
    } catch {
      case e: Calculator.TransactionError =>
        println(s"Error: ${e.getMessage}")
    }
  }
}
