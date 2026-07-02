# Log Analysis Application Guide

This guide will help you set up and use the `log_analysis` application, including log file generation and analysis functionalities. Follow these steps to get started.

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

1. **Clone the repository** containing the `log_analysis` application:

    ```sh
    git clone https://github.com/settorka/Parallel_Patterns_Erlang.git
    cd Parallel_Patterns_Erlang/tests_and_use_cases/pipeline/log_analysis
    ```

2. **Clean and compile** the application using Rebar3:

    ```sh
    rebar3 clean
    rebar3 compile
    ```

## Creating the Log File

Before running the log analysis, you need to create a log file with sample data. Use the `log_file_create` module to generate a log file:

1. **Start the Rebar3 shell**:

    ```sh
    rebar3 shell
    ```

2. **Create a log file with a specified number of records** (e.g., 1000):

    ```erlang
    log_file_create:create_log_file(1000).
    ```

   This command generates `log_file.txt` with 1000 records.

## Running Functions

After creating the log file, you can use the following functions to perform log analysis:

1. **Run the sequential log analysis function**:

   To analyze `log_file.txt` sequentially and write results to `processed_log_file.txt`:

    ```erlang
    log_analysis_sequential:run().
    ```

   This command executes the log analysis sequentially, processing `log_file.txt` and saving the results to `processed_log_file.txt`.

2. **Run the pipeline log analysis function**:

   To analyze `log_file.txt` using a pipeline pattern and write results to `processed_log_file.txt`:

    ```erlang
    log_analysis_pipeline:run().
    ```

   This command executes the log analysis using a pipeline pattern, processing `log_file.txt` and saving the results to `processed_log_file.txt`.

3. **Run the parallel log analysis function**:

   To analyze `log_file.txt` using parallel processing and write results to `processed_log_file.txt`:

    ```erlang
    log_analysis_parallel:run().
    ```

   This command executes the log analysis using parallel processing, processing `log_file.txt` and saving the results to `processed_log_file.txt`.

## Application Overview

### 1. `log_analysis_sequential`

- **Purpose:** Provides sequential log analysis functionality.
- **Key Functions:**
  - `run/0`: Starts the sequential log analysis.
  - `analyze_log/1`: Performs the log analysis stages sequentially.

### 2. `log_analysis_pipeline`

- **Purpose:** Provides log analysis functionality using a pipeline pattern.
- **Key Functions:**
  - `run/0`: Starts the pipeline-based log analysis.
  - `analyze_log/1`: Runs the pipeline with defined stages to process the log data.

### 3. `log_analysis_parallel`

- **Purpose:** Provides log analysis functionality using parallel processing.
- **Key Functions:**
  - `run/0`: Starts the parallel log analysis.
  - `analyze_log/1`: Manages parallel processes to analyze log data.

### 4. `log_file_gen`

- **Purpose:** Generates random log data and writes it to a file.
- **Key Functions:**
  - `create/2`: Generates log data and writes it to a specified file.
  - `generate_log_data/1`: Creates log data in the required format.
  - `generate_records/1`: Generates multiple log records.
  - `generate_record/0`: Creates a single log record with random values.
  - `random_date/0`: Generates a random date.
  - `random_value/2`: Generates a random value within a specified range.
  - `random_time/0`: Generates a random time.
  - `random_sensor/0`: Selects a random sensor name.
  - `write_to_file/2`: Writes data to a specified file.

### 5. `log_file_create`

- **Purpose:** Facilitates the creation of a log file with a given number of lines.
- **Key Functions:**
  - `create_log_file/1`: Calls `log_file_gen:create/2` to generate a log file with the specified number of records.

## Troubleshooting

- Ensure that Erlang and Rebar3 are properly installed by running `erl` and `rebar3` in your terminal. If you encounter issues, verify your installation and paths.
- For detailed error messages or issues with compilation, consult the Rebar3 documentation or seek help in relevant community forums.

---

For further details or contributions, please contact the maintainer:

- **Maintainer:** [amedikusettor@gmail.com](mailto:amedikusettor@gmail.com)

For more information, visit the [GitHub repository](https://github.com/settorka/Parallel_Patterns_Erlang).
