# Multilevel Parallel Patterns

## Project Overview

This project demonstrates the implementation of multilevel parallel pattern libraries using Erlang. It features three versions for each task:

- **Sequential Program:** Baseline implementation executing tasks sequentially.

- **Custom Parallel Program:** Implements parallelization techniques specific to the task.

- **Custom Library Parallel Program:** Utilizes a custom parallel pattern library designed to generalize the parallel pattern for a use case.

### Parallel Pattern Libraries Developed

- Farm

- Pipeline

## Project Structure

The project is organized into two main directories: `text_write` and `log_analysis`. Each directory contains its own set of implementations, Dockerfile, and test scripts.

```

Parallel_Patterns_Erlang/

├── tests_and_use_cases/

    ├── farm/

    │   └── text_write/

    │       ├── Dockerfile

    │       ├── Jenkinsfile

    │       ├── farm_performance_results.csv

    │       ├── requirements.txt

    │       ├── test_performance_farm_local.py

    │       ├── test_performance_farm_cloud.py

    │       ├── performance_plot.png

    │       └── src/

    │           └── (Erlang source files)

    └── pipeline/

        └── log_analysis/

            ├── Dockerfile

            ├── Jenkinsfile

            ├── log_file.txt

            ├── processed_log_file.txt

            ├── requirements.txt

            ├── test_performance_pipeline_local.py

            ├── test_performance_pipeline_cloud.py

            ├── performance_plot.png

            └── src/

                └── (Erlang source files)

```

## Testing

Enter either `text_write` or `log_analysis` directory for farm or pipeline tests respectively.

### Docker Setup

Build the Docker image:

```sh
docker build -t <your_image_name> .
```

### Local Testing

1\. **Run Tests in Docker:**

   After building the Docker image, run tests locally using the Docker container:

   ```
   docker run -d --rm -v $(pwd):/results -w /app <your_image_name>
   ```

   **Explanation:**

   - Runs the Docker container in detached mode

   - Mounts the current directory to `/results`

   - Sets `/app` as the working directory

   - Results will appear in the same folder for analysis and visualization

2\. **Run Tests Without Docker:**

   Alternatively, execute the Python test file directly within your local environment if dependencies are installed:

   ```
   python3 test_performance_pipeline_local.py
   ```

### Cloud Testing

1\. **Prepare Docker for Cloud Deployment:**

   - Log in to Docker in the terminal with your credentials

   - Tag the image with the `latest` tag

   - Push the image to Docker Hub:

    ```sh
     docker tag <your_image_name> <your_dockerhub_username>/<your_image_name>:latest
     docker push <your_dockerhub_username>/<your_image_name>:latest
    ```

2\. **Google Cloud Setup:**

   - **Create Project:** Create a Google account, go to console.cloud.google.com, create a project and billing account

   - **Enable APIs:** Enable the relevant APIs for your Google Cloud project (Batch, Compute)

   - **Create a Cloud Storage Bucket:** Set up a bucket for storing results

3\. **Configure and Run the Batch Job:**

   - Open Google Cloud Batch

   - Create a new job specification:

     - **1 Task, n Parallel:** Set the number of parallel tasks to match the number of CPU cores

     - **Bucket:** Configure the bucket created earlier as the storage for the cluster's distributed file system

   - Monitor the job status, and once complete, retrieve results from the bucket

## Automation with Jenkins

You can automate build, tag, and/or push by:

- Setting up Jenkins

- Authorizing it with GitHub via Personal Access Token

- Creating a pipeline

- Pointing it to the Jenkinsfile

For detailed guides on integration:

- [Jenkins-Github Integration](https://www.youtube.com/watch?v=jSm0YZ-NQAc)

- [Jenkins-Docker Integration](https://www.youtube.com/watch?v=BePJ1bBWk3E&t=908s)

## Summary

This project involves multilevel parallel patterns libraries using Erlang. 
Docker is used for environment setup and cloud deployment. The tests evaluate performance across different batch sizes and parallelization approaches, and results are visualized using Python scripts.

## Contact

For any questions or issues, please contact:

- **Maintainer:** amedikusettor@gmail.com



