import scala.util.Random

object RandomDataGenerator {
  val addresses = List("address1", "address2", "address3", "address4")

  def generateRandomAddress(): String = {
    addresses(Random.nextInt(addresses.length))
  }

  def generateRandomAmount(): Double = {
    Random.nextDouble() * 100.0  // Generates a non-negative random double amount
  }
}
