import subprocess
import csv
import time
import os
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns

# Define the batch sizes
BATCH_SIZES = [10, 100, 750, 5000, 10000,
               50000, 100000, 200000, 500000, 1000000, 2000000, 5000000, 10000000,
               25000000, 60000000, 100000000, 250000000, 500000000, 1000000000]

# Define the output CSV file
CSV_FILE = "/results/pipeline_performance_results_cloud.csv"
PLOT_FILE = "/results/performance_plot_cloud.png"

# Set the working directory to the location where the Erlang modules are compiled
ERLANG_BUILD_PATH = os.path.abspath(
    "_build/default/lib/log_analysis/ebin")  # Update this path

# Define the files to remove
FILES_TO_REMOVE = ["log_file.txt", "processed_log_file.txt"]

# Function to create the log file


def create_log_file(batch_size):
    try:
        print(f"Creating log file with batch size {batch_size}...")

        # Run the Erlang function to create the log file
        subprocess.run(
            ['erl', '-noshell', '-pa', ERLANG_BUILD_PATH, '-eval',
                f'log_file_create:create_log_file({batch_size}), erlang:halt()'],
            stderr=subprocess.PIPE,
            stdout=subprocess.PIPE,
            check=True
        )

        print(f"Log file created with batch size {batch_size}.")

    except subprocess.CalledProcessError as e:
        error_message = e.stderr.decode(
            'utf-8') if e.stderr else 'No error message'
        print(f"Error creating log file: {error_message}")

# Function to run a benchmark and capture results
def run_benchmark(batch_size, approach, module):
    try:
        print(f"Starting {approach} benchmark with batch size {batch_size}...")

        # Record the start time
        start_time = time.time()

        # Run the Erlang function
        result = subprocess.run(
            ['erl', '-noshell', '-pa', ERLANG_BUILD_PATH, '-eval',
                f'{module}:run(), erlang:halt()'],
            stderr=subprocess.PIPE,
            stdout=subprocess.PIPE,
            check=True
        )

        # Record the end time
        end_time = time.time()

        # Calculate the execution time in seconds
        execution_time = round(end_time - start_time,2)

        # Write the results to the CSV file
        with open(CSV_FILE, 'a', newline='') as csvfile:
            writer = csv.writer(csvfile)
            writer.writerow([batch_size, approach, execution_time])

        print(f"Completed {approach} benchmark with batch size {batch_size}.")

    except subprocess.CalledProcessError as e:
        error_message = e.stderr.decode(
            'utf-8') if e.stderr else 'No error message'
        print(f"Error running {module}: {error_message}")


def create_performance_plot(csv_file: str, output_file: str):
    # Read the CSV file into a DataFrame
    df = pd.read_csv(csv_file)

    # Set up the matplotlib figure
    plt.figure(figsize=(14, 8))

    # Plot the execution times for each approach with dots and lines
    sns.lineplot(data=df, x='BatchSize', y='Time(sec)',
                 hue='Approach', marker='o', linestyle='-')

    # Add title and labels
    plt.title('Execution Time by Batch Size and Approach')
    plt.xlabel('Batch Size')
    plt.ylabel('Time (seconds)')
    plt.legend(title='Approach')

    # Add grid for better readability
    plt.grid(True, which="both", linestyle="--", linewidth=0.5)

    # Save the plot as an image file
    plt.savefig(output_file)

    # Show the plot
    plt.show()


def cleanup_files(files):
    for file in files:
        try:
            if os.path.exists(file):
                os.remove(file)
                print(f"Removed file: {file}")
            else:
                print(f"File not found: {file}")
        except Exception as e:
            print(f"Error removing file {file}: {e}")


def main():
    # Create or overwrite the CSV file and write headers
    with open(CSV_FILE, 'w', newline='') as csvfile:
        writer = csv.writer(csvfile)
        writer.writerow(["BatchSize", "Approach", "Time(sec)"])

    # Iterate over batch sizes and approaches
    for batch_size in BATCH_SIZES:
        create_log_file(batch_size)
        run_benchmark(batch_size, "sequential", "log_analysis_sequential")
        run_benchmark(batch_size, "parallel", "log_analysis_parallel")
        run_benchmark(batch_size, "pipeline", "log_analysis_pipeline")

    print(f"Benchmarking completed. Results saved to {CSV_FILE}.")
    # Call the visualization function
    create_performance_plot(CSV_FILE, PLOT_FILE)

    # Clean up specific files
    cleanup_files(FILES_TO_REMOVE)


if __name__ == "__main__":
    main()
