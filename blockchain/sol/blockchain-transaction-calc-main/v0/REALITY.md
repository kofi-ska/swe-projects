## Blockchain Transaction Calc

### Overview

The calculator in Scala manages transactions involving accounts identified by addresses. Here are the key functionalities:

#### addAmount Function

This function adds a specified amount to an account identified by its address.
If the address exists, it updates the existing amount; if not, it creates a new entry.

#### subtractAmount Function

This function subtracts a specified amount from an account identified by its address and returns an Either type:

- Right(accounts) if successful and the resulting amount is non-negative.
- Left(TransactionError) if there is an issue, such as insufficient funds or attempting to subtract from a non-existing account.

### Implementation Details

#### Functional Approach

Functions are implemented functionally in Scala using immutable data structures like Map for thread safety and clarity.

#### Error Handling

Error scenarios are managed using Scala's Exception and Either types to ensure clear error messages and prevent crashes.

#### Scala Features Used

- **Case Classes:** TransactionError is defined as a case class for immutable error handling.
- **Pattern Matching:** Used extensively in subtractAmount for handling different scenarios.
- **Immutable Collections:** Scala's immutable Map ensures data integrity.

### Function Testing

Unit tests validate addAmount and subtractAmount from the Calculator object with specific objectives:

- **addAmount Function Tests:** Ensure correct addition and creation of entries.
- **subtractAmount Function Tests:** Validate accurate subtraction and error handling for insufficient funds or non-existing accounts.
- **Edge Cases Handled:** Tests for scenarios like non-existing addresses and insufficient funds.

### Production Capacity Planning

An integration test evaluates the calculator's performance under production conditions with concurrent transactions to simulate multiple hits from a frontend applicaiton:

- **Design Choice:** Utilizes Scala's concurrency features for scalability and efficiency.
- **Optimized Resource Utilization:** Multithreading to handle simultaneous transactions effectively.
- **Realistic Load Simulation:** Mimics API hits across distributed nodes.

### Production Testing

Components include:

- **RandomDataGenerator:** Generates random addresses and amounts.
- **MultithreadedCompute:** Executes concurrent computations across multiple threads.
- **IntegrationTest:** Validates performance under load using Scala's asynchronous capabilities.

### Running Instructions

Install:

**sbt** https://www.scala-sbt.org/1.x/docs/Setup.html
**scala** https://www.scala-lang.org/download/

```shell
# Clone this repository
# Navigate to project folder in terminal and execute:
$ sbt clean compile
# Sample use of application
$ sbt run
#Functional and production tests for application
$ sbt "*testOnly UnitTests.scala"
$ sbt "*testOnly IntegrationTest.scala"

```
