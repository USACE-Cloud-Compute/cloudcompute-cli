# Cloud Compute Command Line Interface (cccli)

The Cloud Compute Command Line Interface (cccli) is a powerful tool for registering and running cloud computations across multiple compute providers. It supports Docker, AWS Batch, Kubernetes, and Kubernetes Argo Workflows as compute providers.

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Configuration](#configuration)
  - [Compute File](#compute-file)
  - [Plugin Manifest](#plugin-manifest)
  - [Compute Manifest](#compute-manifest)
  - [Environment File](#environment-file)
- [Usage](#usage)
  - [Commands](#commands)
- [Event Generators](#event-generators)
- [Quick Start Example](#quick-start-example)
- [Building from Source](#building-from-source)
- [License](#license)

## Overview

cccli orchestrates cloud compute jobs through a flexible configuration system based on JSON files. Jobs are defined by three key components:

- **Compute File**: Defines the job name, compute provider, plugins, and event structure
- **Plugin Manifest**: Specifies the container image, resources, and execution details
- **Compute Manifest**: Defines the specific commands and parameters for each job execution

## Installation

Download the latest release from the [GitHub repository](https://github.com/USACE-Cloud-Compute/cc-cli) or build from source (see [Building from Source](#building-from-source)).

## Configuration

### Compute File

The compute file is the main configuration that defines your cloud compute job. Example:

```json
{
    "name": "Hello World Test",
    "provider": {
        "type": "docker",
        "concurrency": 1,
        "queue": "docker-local"
    },
    "plugins": ["hello-world-plugin.json"],
    "event": {
        "compute-manifests": ["hello-world-manifest1.json", "hello-world-manifest2.json"]
    }
}
```

**Supported Provider Types:**
- `docker` - Local Docker provider (auto-registers plugins)
- `awsbatch` - AWS Batch
- `k8s` - Kubernetes
- `k8sargo` - Kubernetes Argo Workflows

### Plugin Manifest

Defines the container image and compute resources:

```json
{
    "name": "hello-world-plugin",
    "image_and_tag": "ghcr.io/usace-cloud-compute/hello-world-plugin:latest",
    "description": "Docker Hello World",
    "command": ["/runme"],
    "compute_environment": {
        "vcpu": "1",
        "memory": "256"
    }
}
```

### Compute Manifest

Specifies the execution details for each job:

```json
{
    "plugin_definition": "hello-world-plugin",
    "command": ["sleep", "10"],
    "depends-on": ["other-manifest.json"]
}
```

The `depends-on` field allows you to specify dependencies between compute manifests within an event.

### Environment File

Environment variables provide credentials and configuration for the compute provider:

```bash
CC_AWS_ACCESS_KEY_ID=your_access_key
CC_AWS_SECRET_ACCESS_KEY=your_secret_key
CC_AWS_DEFAULT_REGION=us-east-1
CC_AWS_S3_BUCKET=compute-store
CC_AWS_ENDPOINT=http://localhost:9000
```

For AWS Batch, use standard AWS credentials. For local development with MinIO (S3-compatible storage), use the MinIO endpoint.

## Usage

Basic command structure:

```bash
cccli [global-flags] <command> [command-flags] [arguments]
```

### Global Flags

- `-c, --computeFile` - Path to compute configuration file (default: `compute.json`)
- `-e, --envFile` - Path to environment file for provider credentials

### Commands

#### `run`

Execute a compute job defined in the compute file.

```bash
cccli -c compute.json -e .env run [flags]
```

**Flags:**
- `-j, --jobStore` - Path to CSV file for storing job identifiers
- `-p, --eventPostProcessor` - Path to JavaScript file for post-processing events

**Example:**

```bash
cccli -c data/hello-world/compute.json -e .env run --jobStore=jobs.csv
```

The job store CSV contains:
- Compute ID (Cloud Compute identifier)
- Event ID (Cloud Compute identifier)  
- Job ID (Cloud Compute identifier)
- ComputeProviderJob (Provider-specific job ID)
- Payload GUID (Storage location for job data)
- Event Identifier (String identifier for the event)

#### `register`

Register plugin(s) with the compute provider. By default, registers all plugins referenced in the compute file.

```bash
cccli -c compute.json -e .env register [plugin-manifest-file]
```

**Examples:**

```bash
# Register all plugins from compute file
cccli -c data/hello-world/compute.json -e .env register

# Register a single plugin manifest
cccli -e .env register path/to/plugin-manifest.json
```

#### `terminate`

Terminate running jobs at different levels (compute, event, or individual job).

```bash
cccli -c compute.json -e .env terminate <level> <id> <message>
```

**Arguments:**
- `level` - Termination scope: `COMPUTE`, `EVENT`, or `JOB`
- `id` - GUID of the compute/event/job to terminate
- `message` - Reason for termination (recorded by provider)

**Example:**

```bash
cccli -c compute.json -e .env terminate COMPUTE 068ff6fe-d8d9-48af-b897-b937a7e14dae "Cancelling test run"
```

#### `log`

Retrieve logs from a completed or running job.

```bash
cccli -c compute.json -e .env log <job-id>
```

**Arguments:**
- `job-id` - Provider-specific job identifier

**Example:**

```bash
cccli -c compute.json -e .env log 20e84f68-56dc-46fd-b5f3-c2494e4c5f83
```

#### `status`

Query job status summaries at different levels.

```bash
cccli -c compute.json -e .env status <level> <id>
```

**Arguments:**
- `level` - Query level: `COMPUTE`, `EVENT`, or `JOB`
- `id` - Identifier to query

**Example:**

```bash
cccli -c compute.json -e .env status COMPUTE 068ff6fe-d8d9-48af-b897-b937a7e14dae
```

Output format: `JobId, StartedAt, StoppedAt, Status, StatusDetail`

#### `schema`

Export JSON schemas for validation and documentation.

```bash
cccli schema <schema-type>
```

**Arguments:**
- `schema-type` - Type of schema: `compute`, `plugin`, or path to compute manifest file

**Examples:**

```bash
# Export compute manifest schema
cccli schema compute

# Export plugin manifest schema
cccli schema plugin

# Generate schema from existing compute file
cccli schema data/hello-world/compute.json
```

#### `version`

Display version information.

```bash
cccli version
```

## Event Generators

Event generators create multiple job executions from a single compute configuration. Configure generators in the compute file.

### Array Generator

Generate events based on a numeric range:

```json
{
    "generator": {
        "type": "array",
        "start": 1,
        "end": 2,
        "perEventLoop": [
            {"ENV1": "value1", "ENV2": "value2"},
            {"ENV1": "value3", "ENV2": "value4"}
        ]
    }
}
```

This configuration will generate 4 jobs total (2 events × 2 perEventLoop objects). Each job will have `CC_EVENT_NUMBER` set to "1" or "2", plus the environment variables from its perEventLoop object.

### Array Generator 2

Enhanced array generator with additional environment variables:

```json
{
    "generator": {
        "type": "array2",
        "start": 1,
        "end": 100,
        "addEnv": {
            "CUSTOM_VAR": "custom_value"
        },
        "perEventLoop": [
            {"ENV1": "value1", "ENV2": "value2"}
        ]
    }
}
```

### Stream Generator

Generate events from CSV or delimited file data:

```json
{
    "generator": {
        "type": "stream",
        "file": "data/events.csv",
        "delimiter": ",",
        "perEventLoop": [
            {"ENV1": "value1", "ENV2": "value2"}
        ]
    }
}
```

Each row in the CSV file becomes an event. 

### Realization-Block Generator

Generate events for combinations of realizations and blocks:

```json
{
    "generator": {
        "type": "realzblock",
        "startRealz": 1,
        "endRealz": 100,
        "startBlock": 1,
        "endBlock": 50
    }
}
```

The `addEnv` field adds static environment variables to all jobs, while `perEventLoop` creates job variations.

### Per-Event Loop

The `perEventLoop` parameter is an array of objects that multiplies the number of jobs submitted for each event. For each event generated, cccli will:

1. Enumerate through each object in the `perEventLoop` array
2. Add that object's key-value pairs to the job's environment variables
3. Submit a separate job with those environment variables

**Example:** An array event generator with `start: 1`, `end: 2`, and a `perEventLoop` array with 2 objects will submit **4 total jobs**:
- Event 1 with perEventLoop object 1
- Event 1 with perEventLoop object 2
- Event 2 with perEventLoop object 1
- Event 2 with perEventLoop object 2

This is useful for running parameter sweeps, multiple configurations, or parallel processing variations of the same computational task.

## Quick Start Example

This example uses Docker and MinIO (local S3-compatible storage) to run a simple "Hello World" computation.

### 1. Start MinIO

```bash
docker compose up
```

### 2. Configure MinIO

1. Open `http://localhost:9001` in your browser
2. Login with credentials from `docker-compose.yml`
3. Create a bucket named `compute-store`
4. Create access keys under **User > Access Keys**
5. Create a path `cc_store` in the bucket under **Object Browser**

### 3. Create Environment File

Create `.env` with your MinIO credentials:

```bash
CC_AWS_ACCESS_KEY_ID=your_minio_access_key
CC_AWS_SECRET_ACCESS_KEY=your_minio_secret_key
CC_AWS_DEFAULT_REGION=us-east-1
CC_AWS_S3_BUCKET=compute-store
CC_AWS_ENDPOINT=http://localhost:9000
```

### 4. Run the Example

```bash
cccli -c data/hello-world/compute.json -e .env run
```

The Docker provider automatically registers plugins before execution and waits for all jobs to complete.

## Building from Source

### Prerequisites

- Go 1.25.0 or later
- Git

### Build Commands

```bash
cd src

# Basic build
go build -o cccli ./cmd

# Build with version information
go build -ldflags "-X main.version=v1.0.0 -X main.commit=$(git describe --tags --always --long) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o cccli ./cmd
```

The compiled binary will be created as `cccli` in the src directory.

## Notes

- Docker provider automatically registers plugins before running jobs
- Docker provider waits for all jobs to complete before exiting
- Job manifests can specify dependencies using the `depends-on` field
- Multiple compute providers can be configured for different environments
- Event post-processors (JavaScript) allow custom modification of event data

## License

MIT License - Copyright (c) 2025 USACE

See [LICENSE](LICENSE) file for details.

