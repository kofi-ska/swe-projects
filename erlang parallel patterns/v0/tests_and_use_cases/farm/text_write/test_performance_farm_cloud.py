import subprocess
import csv
import time
import os
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns

# Define the batch sizes
BATCH_SIZES = [10, 100, 750, 5000, 10000,
               50000, 100000, 200000, 500000, 1000000, 2000000, 4500000, 
               10000000, 17500000, 30000000, 50000000, 75000000, 100000000,
               225000000, 500000000, 750000000, 1000000000]


# Define the output CSV file and the output plot file
CSV_FILE = "/results/farm_performance_results_cloud.csv"
PLOT_FILE = "/results/performance_plot_cloud.png"

# Define the files to remove
FILES_TO_REMOVE = ["output.txt", "output_parallel.txt", "output_farm.txt"]

# Set the working directory to the location where the Erlang modules are compiled
ERLANG_BUILD_PATH = os.path.abspath("_build/default/lib/text_write/ebin")

# Function to run a benchmark and capture results
def run_benchmark(batch_size, approach, module):
    try:
        print(f"Starting {approach} benchmark with batch size {batch_size}...")

        # Record the start time
        start_time = time.time()

        # Run the Erlang function
        result = subprocess.run(
            ['erl', '-noshell', '-pa', ERLANG_BUILD_PATH, '-eval',
                f'{module}:run({batch_size}), erlang:halt()'],
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
        run_benchmark(batch_size, "sequential", "text_write_sequential")
        run_benchmark(batch_size, "parallel", "text_write_parallel")
        run_benchmark(batch_size, "farm", "text_write_farm")

    print(f"Benchmarking completed. Results saved to {CSV_FILE}.")

    # Call the visualization function
    create_performance_plot(CSV_FILE, PLOT_FILE)

    # Clean up specific files
    cleanup_files(FILES_TO_REMOVE)


if __name__ == "__main__":
    main()
