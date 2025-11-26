# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

- `make build` - builds the lord binary from go source
- `make install` - installs lord binary to /usr/local/bin (requires sudo)
- `make clean` - removes the lord binary
- `make test` - runs all unit tests
- `make format` - formats all go source files
- `go build -o lord .` - direct go build command

## Testing

Unit tests are present in the codebase for core functionality that doesn't require SSH/remote operations:
- **traefik_test.go**: comprehensive tests for traefik configuration management, including YAML serialization/deserialization and timeout update logic

All other testing is done through manual integration testing with real deployments.

## Code Architecture

Lord is a minimalist PaaS management service written in Go that builds and deploys Docker containers to remote Linux hosts via SSH.

### Core Components

- **main.go**: CLI entry point with flag parsing for all commands (deploy, logs, init, server, destroy, status, monitor, dozzle, etc.)
- **config.go**: Configuration management using Viper, loads lord.yml files with deployment settings
- **local.go**: Local operations including Docker build/push, project initialization, and command execution utilities
- **remote.go**: Remote server operations via SSH including Docker management, container deployment, and Traefik proxy setup
- **traefik.go**: Traefik reverse proxy configuration and management for web containers
- **ssh_utils.go**: SSH connection utilities and remote command execution
- **system_monitor.go**: System monitoring functionality for CPU, memory, storage, and Docker stats
- **dozzle.go**: Dozzle container monitoring UI integration with SSH tunneling
- **registry.go**: Container registry authentication and management (AWS ECR, DigitalOcean)

### Key Configuration

Projects require a `lord.yml` file with deployment settings:
- Container registry and authentication
- Target server details
- Optional volumes, environment files, build args
- Web service configuration (defaults to port 80)
- Advanced web configuration (WebAdvancedConfig):
  - Timeout settings: readTimeout, writeTimeout, idleTimeout (global Traefik settings)
  - Buffering settings: maxRequestBodyBytes, maxResponseBodyBytes, memRequestBodyBytes (per-service)

Multi-config support: Use `config.lord.yml` files and `-config config` flag for multiple deployment targets.

### Deployment Flow

1. Build Docker container locally with specified platform
2. Push to configured registry OR save/transfer directly (registry-less deployment)
3. SSH to target server and ensure Docker/Traefik setup
4. If WebAdvancedConfig timeouts are set and Traefik is already running:
   - Read existing Traefik config from /etc/traefik/traefik.yml
   - Apply "maximum wins" logic: update timeouts only if new values are higher
   - Restart Traefik container if config was updated
5. Pull container and run with configured volumes/settings
6. For web containers, configure Traefik routing with optional buffering middleware

### Registry Support

- **Direct deployment**: Save/load containers without registry
- **AWS ECR**: Dynamic authentication via AWS credentials
- **DigitalOcean**: Dynamic authentication via DO API token
- **Generic registries**: Manual auth via config.json file

The architecture emphasizes simplicity with minimal configuration options and dependencies.