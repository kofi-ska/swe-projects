# Use an official Erlang base image
FROM erlang:26

# Install rebar3, Python, and virtualenv
RUN apt-get update && apt-get install -y \
    rebar3 \
    python3-pip \
    python3-venv \
    && apt-get clean

# Set the working directory inside the container
WORKDIR /app

# Copy the current directory contents into the container
COPY . .

# Create a virtual environment and activate it
RUN python3 -m venv /venv
ENV PATH="/venv/bin:$PATH"

# Install Python dependencies
RUN pip install --upgrade pip && pip install -r requirements.txt

# Compile the Erlang application
RUN rebar3 compile

# Define a volume to store the results
VOLUME ["/results"]

# Command to run the benchmarks and then visualize the results
CMD ["python3","test_performance_pipeline_cloud.py"]
