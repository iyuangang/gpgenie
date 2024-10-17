# GPGenie

<!--![gpgenie Logo](path/to/logo.png)  todo: Add a logo for gpgenie-->

**GPGenie** is a robust Go-based tool designed for generating, scoring, and managing GPG keys. It leverages powerful libraries like GORM for database interactions and Zap for efficient logging. With configurable settings, gpgenie ensures flexibility and scalability to accommodate various use cases.

## Table of Contents

- [GPGenie](#gpgenie)
  - [Table of Contents](#table-of-contents)
  - [Features](#features)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
    - [Clone the Repository](#clone-the-repository)
    - [Build from Source](#build-from-source)
  - [Configuration](#configuration)
    - [Sample `config.json`](#sample-configjson)
    - [Configuration Parameters](#configuration-parameters)
  - [Usage](#usage)
    - [Running the Application](#running-the-application)
    - [Command-Line Arguments](#command-line-arguments)
    - [Examples](#examples)
  - [Docker](#docker)
    - [Building the Docker Image](#building-the-docker-image)
    - [Running with Docker](#running-with-docker)
    - [Example Commands](#example-commands)
    - [Docker Compose (Optional)](#docker-compose-optional)
  - [Contributing](#contributing)
  - [License](#license)
  - [Acknowledgements](#acknowledgements)

## Features

- **GPG Key Generation**: Create robust GPG key pairs with configurable parameters.
- **Scoring Mechanism**: Evaluate keys based on custom scoring algorithms.
- **Database Management**: Store and manage key details using PostgreSQL or SQLite.
- **Encryption**: Secure private keys with encryption mechanisms.
- **Concurrency**: Efficiently generate multiple keys concurrently.
- **Export Functionality**: Export keys and key data in CSV and encrypted formats.
- **Logging**: Comprehensive logging using Zap for monitoring and debugging.

## Prerequisites

- **Go**: Version 1.20 or higher.
- **Docker**: For containerization (optional but recommended).
- **Database**: PostgreSQL or SQLite.
- **Public Key**: For encrypting private keys (optional based on configuration).

## Installation

### Clone the Repository

```bash
git clone https://github.com/iyuangang/gpgenie.git
cd gpgenie
```

### Build from Source

Ensure you have Go installed and set up properly.

```bash
go build -o gpgenie ./cmd/gpgenie
```

This will generate an executable named `gpgenie` in your current directory.

## Configuration

gpgenie uses a JSON configuration file to manage settings. By default, it looks for `config/config.json`. You can specify a different path using the `-config` flag.

### Sample `config.json`

```json
{
  "database": {
    "type": "postgres",
    "host": "localhost",
    "port": 5432,
    "user": "youruser",
    "password": "yourpassword",
    "dbname": "gpgenie_db",
    "max_open_conns": 25,
    "max_idle_conns": 25,
    "conn_max_lifetime": 300
  },
  "processing": {
    "batch_size": 100
  },
  "key_generation": {
    "total_keys": 1000,
    "num_workers": 10,
    "min_score": 50,
    "max_letters_count": 10,
    "name": "John Doe",
    "comment": "gpgenie Key",
    "email": "johndoe@example.com"
  },
  "key_encryption": {
    "public_key_path": "path/to/public_key.asc"
  }
}
```

### Configuration Parameters

- **Database Configuration**
  - `type`: Database type (`postgres` or `sqlite`).
  - `host`: Database host (only for PostgreSQL).
  - `port`: Database port (only for PostgreSQL).
  - `user`: Database user (only for PostgreSQL).
  - `password`: Database password (only for PostgreSQL).
  - `dbname`: Database name.
  - `max_open_conns`: Maximum open connections.
  - `max_idle_conns`: Maximum idle connections.
  - `conn_max_lifetime`: Connection maximum lifetime in seconds.

- **Processing Configuration**
  - `batch_size`: Number of keys processed in each batch.

- **Key Generation Configuration**
  - `total_keys`: Total number of GPG keys to generate.
  - `num_workers`: Number of concurrent workers for key generation.
  - `min_score`: Minimum score a key must have to be considered valid.
  - `max_letters_count`: Maximum number of unique letters allowed in a key.
  - `name`: Name for the GPG entity.
  - `comment`: Comment for the GPG entity.
  - `email`: Email for the GPG entity.

- **Key Encryption Configuration**
  - `public_key_path`: Path to the public key used for encrypting private keys.

## Usage

### Running the Application

```bash
./gpgenie -config config/config.json [flags]
```

### Command-Line Arguments

- `-config`: Path to the configuration file. Default is `config/config.json`.
- `-generate-keys`: Generate GPG keys based on the configuration.
- `-show-top N`: Display the top N keys by score.
- `-show-low-letter N`: Display the top N keys with the lowest letter count.
- `-export-by-fingerprint FP`: Export a key by the last 16 characters of its fingerprint.
- `-output-dir DIR`: Specify the output directory for exported keys. Default is the current directory.

### Examples

- **Generate Keys**

  ```bash
  ./gpgenie -config config/config.json -generate-keys
  ```

- **Show Top 10 Keys by Score**

  ```bash
  ./gpgenie -config config/config.json -show-top 10
  ```

- **Show Top 5 Keys with Lowest Letter Count**

  ```bash
  ./gpgenie -config config/config.json -show-low-letter 5
  ```

- **Export a Key by Fingerprint**

  ```bash
  ./gpgenie -config config/config.json -export-by-fingerprint ABCDEF1234567890 -output-dir /path/to/export
  ```

## Docker

Containerizing gpgenie using Docker ensures consistent environments and simplifies deployment.

### Building the Docker Image

1. **Create a Dockerfile** in the root directory (already provided below).

2. **Build the Image**

   ```bash
   docker build -t gpgenie:latest .
   ```

### Running with Docker

Ensure you have your configuration file (`config.json`) ready and accessible.

```bash
docker run --rm \
  -v /path/to/your/config:/app/config \
  -v /path/to/export:/output \
  gpgenie:latest \
  -config /app/config/config.json [flags]
```

### Example Commands

- **Generate Keys**

  ```bash
  docker run --rm \
    -v /path/to/your/config:/app/config \
    gpgenie:latest \
    -config /app/config/config.json -generate-keys
  ```

- **Show Top 10 Keys by Score**

  ```bash
  docker run --rm \
    -v /path/to/your/config:/app/config \
    gpgenie:latest \
    -config /app/config/config.json -show-top 10
  ```

- **Export a Key by Fingerprint**

  ```bash
  docker run --rm \
    -v /path/to/your/config:/app/config \
    -v /path/to/export:/output \
    gpgenie:latest \
    -config /app/config/config.json -export-by-fingerprint ABCDEF1234567890 -output-dir /output
  ```

### Docker Compose (Optional)

If your application depends on a PostgreSQL database, Docker Compose can help orchestrate multi-container setups.



**Running with Docker Compose**

```bash
docker-compose up --build
```

This will start both the PostgreSQL database and gpgenie application services.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request with your improvements.

1. Fork the repository.
2. Create a new branch: `git checkout -b feature/YourFeature`.
3. Commit your changes: `git commit -am 'Add some feature'`.
4. Push to the branch: `git push origin feature/YourFeature`.
5. Open a pull request.

## License

This project is licensed under the [MIT License](LICENSE).

## Acknowledgements

- [Go](https://golang.org/)
- [GORM](https://gorm.io/)
- [Viper](https://github.com/spf13/viper)
- [Zap Logger](https://github.com/uber-go/zap)
- [OpenPGP](https://github.com/ProtonMail/go-crypto)
