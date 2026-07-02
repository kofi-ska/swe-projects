# Library Guide

This guide will help you develop and test the Erlang library for parallel patterns and distributed processing engine bindings. Follow these steps to get started.

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

1. **Clone the repository** containing the Erlang apps:

   ```sh
   git clone https://github.com/settorka/Parallel_Patterns_Erlang.git
   cd Parallel_Patterns_Erlang/lib
   ```

2. **Navigate to the specific app folder** you want to work on. For example, to work with the `pipeline_parallel_pattern` app:

   ```sh
   cd pipeline_parallel_pattern
   ```

3. **Clean, compile, and run tests** for the app using Rebar3:

   ```sh
   rebar3 clean
   rebar3 compile
   rebar3 eunit
   ```

Repeat step 2 for `farm_parallel_pattern` and `dpe_bindings` as needed.

## Application Overview

### 1. `pipeline_parallel_pattern`

- **Purpose:** Implements a pipeline parallel pattern for processing data through a sequence of stages.
- **Key Functions:**
  - `run_pipeline/2`: Starts the pipeline processing with the given stages and initial input.

### 2. `farm_parallel_pattern`

- **Purpose:** Implements a farm parallel pattern for distributing workloads across multiple workers.
- **Key Functions:**
  - `farm_work/4`: Distributes the workload among workers and waits for their completion.
  - `create_or_overwrite_file/1`: Creates or overwrites a file.

### 3. `dpe_bindings`

- **Purpose:** Provides Erlang bindings for interacting with distributed processing engines like Spark and Flink.
- **Key Functions:**
  - `submit_spark_job/3`: Submits a job to Spark.
  - `submit_flink_job/3`: Submits a job to Flink.

## Development and Testing

- **Test Adaptation:** The applications have been adapted for testing in the `test_and_use_cases` folder. The primary development occurs in the `lib` folder as described above.

## Troubleshooting

- Ensure that Erlang and Rebar3 are properly installed by running `erl` and `rebar3` in your terminal. If you encounter issues, verify your installation and paths.
- For detailed error messages or issues with compilation and testing, consult the Rebar3 documentation or seek help in relevant community forums.

---

For further details or contributions, please contact the maintainer:

- **Maintainer:** [amedikusettor@gmail.com](mailto:amedikusettor@gmail.com)

For more information, visit the [GitHub repository](https://github.com/settorka/Parallel_Patterns_Erlang).
