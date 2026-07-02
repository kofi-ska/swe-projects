# Text Write Application Guide

This guide will help you set up and use the `text_write` application, which includes both sequential and parallel file writing functionalities. Follow these steps to get started.

## Prerequisites

1. **Install WSL (Windows Subsystem for Linux)** if you are using Windows. For Linux and macOS, you can skip this step.
2. **Install Ubuntu** (preferred) or another Linux distribution, or use macOS.

## Installing Erlang and Rebar3

### On Ubuntu/Linux:

1. Update the package list and install Erlang and Rebar3:

    ```sh
    sudo apt update
    sudo apt install erlang rebar3
    ```

2. For macOS, use Homebrew:

    ```sh
    brew install erlang rebar3
    ```

## Setting Up the Project

1. **Clone the repository** containing the `text_write` application:

    ```sh
    git clone https://github.com/settorka/Parallel_Patterns_Erlang.git
    cd Parallel_Patterns_Erlang/tests_and_use_cases/farm/text_write
    ```

2. **Clean and compile** the application using Rebar3:

    ```sh
    rebar3 clean
    rebar3 compile
    ```

## Running Functions

You can use the Rebar3 shell to interact with the application and run the provided functions.

1. **Start the Rebar3 shell**:

    ```sh
    rebar3 shell
    ```

2. **Run the sequential file writing function**:

   To write "Hello, World!" to `output.txt` a specified number of times:

    ```erlang
    text_write_sequential:run(10).
    ```

   This command will create or overwrite `output.txt` and write "Hello, World!" 10 times.

3. **Run the parallel file writing function**:

   To write "Hello, World!" to `output_parallel.txt` using parallel processing:

    ```erlang
    text_write_parallel:run(10).
    ```

   This command will create or overwrite `output_parallel.txt` and write "Hello, World!" 10 times using multiple cores.

4. **Run the file writing function using a farm pattern**:

   To write "Hello, World!" to `output_farm.txt` using the farm parallel pattern:

    ```erlang
    text_write_farm:run(10).
    ```

   This command will create or overwrite `output_farm.txt` and write "Hello, World!" 10 times using the farm parallel pattern.

## Application Overview

### 1. `text_write_sequential`

- **Purpose:** Provides sequential file writing functionality.
- **Key Functions:**
  - `create_or_overwrite_file/1`: Creates or overwrites the specified file.
  - `write_to_file/3`: Writes a specified text to the file a number of times.
  - `run/1`: Executes the file writing operation with a given number of times.

### 2. `text_write_parallel`

- **Purpose:** Provides parallel file writing functionality using the number of logical processors.
- **Key Functions:**
  - `create_or_overwrite_file/1`: Creates or overwrites the specified file.
  - `write_to_file_parallel/3`: Writes a specified text to the file in parallel.
  - `run/1`: Executes the parallel file writing operation with a given number of times.

### 3. `text_write_farm`

- **Purpose:** Provides file writing functionality using a farm parallel pattern.
- **Key Functions:**
  - `file_write_worker/2`: Worker function for writing text to the file.
  - `run/1`: Executes the file writing operation using the farm parallel pattern.

## Troubleshooting

- Ensure that Erlang and Rebar3 are properly installed by running `erl` and `rebar3` in your terminal. If you encounter issues, verify your installation and paths.
- For detailed error messages or issues with compilation, consult the Rebar3 documentation or seek help in relevant community forums.

---

For further details or contributions, please contact the maintainer:

- **Maintainer:** [amedikusettor@gmail.com](mailto:amedikusettor@gmail.com)

For more information, visit the [GitHub repository](https://github.com/settorka/Parallel_Patterns_Erlang).
