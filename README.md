# Lord

A minimalist PaaS management service for deploying Docker containers to remote hosts.

Lord builds your Docker containers locally and deploys them to Linux servers via SSH. It focuses on simplicity with minimal configuration and zero dependencies on the target server beyond Docker.

## Quick Start

Lord requires these things to get started:

1. A `Dockerfile` in your project directory
2. A `lord.yml` configuration file
3. A container registry (optional - you can use registry-less deployment)

Get started by running `lord -init` in your project directory to generate a configuration template.

## How It Works

1. **Build**: Lord builds your Docker container locally with the specified platform
2. **Push/Transfer**: Container is either pushed to your configured registry or transferred directly via SFTP (registry-less)
3. **Deploy**: Lord SSHs to your server and pulls/loads/runs the container
4. **Proxy**: Web services are automatically configured with Traefik reverse proxy for https

### Container Conventions

- Web services must expose port 80 internally
- Persistent data should use the `/data` volume mount
- Additional volumes can be specified in configuration

## Commands

```sh
lord -init         # create lord.yml configuration file
lord -deploy       # build and deploy your application
lord -logs         # stream container logs from server
lord -destroy      # remove deployed containers
lord -status       # check deployment status
lord -server       # only run and/or check the server setup (includes reverse proxy)
lord -proxy        # only run and/or check the reverse proxy setup
lord -recover      # attempt to recover a server that has a bad install/setup of lord dependencies
lord -logdownload  # download a full log file from the server
lord -registry     # only setup and authenticate to the container registry
lord -dozzle       # run the dozzle ui locally connected to the remote container
```

## Configuration

Create a `lord.yml` file in your project directory:

```yaml
# required fields
name: myapp                           # unique app name per host
server: 192.168.1.100                 # target server ip address

# registry configuration (optional)
registry: my.realregistry.com/me      # container registry url (omit for registry-less deployment)
authfile: ./config.json               # docker registry auth file

# optional fields
email: user@example.com               # email for tls certificates
platform: linux/amd64                 # build platform (default: linux/amd64)
target: production                    # docker build target stage
web: true                             # enable web service with traefik
hostname: myapp.example.com           # domain name (required if web: true)
environmentfile: .env                 # environment variables file
buildargfile: build.args              # docker build arguments file
hostenvironmentfile: host.env         # host environment variables file
user: ubuntu                          # ssh login user (default: root)
sshkeyfile: /path/to/private/key      # custom ssh private key file

# additional volume mounts (follows docker format)
volumes:
  - /host/data:/container/data
  - /etc/config:/app/config
```

### Supporting Multiple Applications/Containers

Lord supports multiple `lord.yml` files in a single repository in cases where:

- There are multiple containers/variants that can be built from a single Dockerfile (i.e. `--target`)
- There are multiple remote hosts that the container is deployed to

To achieve this, each separate Lord config file can be prefixed with a unique config key using dot notation.

For example, using the following file naming:

```
project dir ─┐
             ├─── Dockerfile
             ├─── conf1.lord.yml
             └─── conf2.lord.yml
```
The following Lord config variants can be used:

- `conf1`
- `conf2`

To perform actions against each, the `-config` flag can be included in the Lord command along with the config key:

``` sh
lord -config conf2 -deploy
```

### Host Environment Variables

The `hostenvironmentfile` field allows you to specify environment variables that will be available on the remote host during all application-specific commands. This is useful for:

- Registry authentication credentials (AWS_ACCESS_KEY_ID, GOOGLE_APPLICATION_CREDENTIALS)
- Cloud provider credentials for container pulls
- Application-specific secrets that need to be available during deployment

The specified file will be copied to `/etc/lord/{appname}` on the remote host and automatically sourced before executing Docker commands for your application. Each application maintains its own environment file, allowing different apps on the same host to have different environment variables.

Example host environment file:
```bash
export AWS_ACCESS_KEY_ID=your_access_key
export AWS_SECRET_ACCESS_KEY=your_secret_key
export CUSTOM_DEPLOY_TOKEN=your_token
```

### Registry Authentication

Lord supports two methods for registry authentication:

#### Method 1: Auth File (Manual)
Provide a `config.json` file with registry authentication that will be copied to your server:

```json
{
  "auths": {
    "my.realregistry.com": {
      "auth": "base64encodedcredentials"
    }
  }
}
```

**NOTE:** this method overwrites the entire `.docker/config.json` file on the host. In order to use this
method, all containers on the host must share the same registry and authentication method.

#### Method 2: Dynamic Authentication (Recommended)
Lord can automatically authenticate to supported registries using environment variables. Currently supported:

- **AWS ECR** - requires AWS credentials
- **Digital Ocean Container Registry** - requires Digital Ocean API token

When no `authfile` is specified in `lord.yml`, Lord will attempt dynamic authentication based on the registry URL.

##### AWS ECR Authentication
For ECR registries (URLs containing `amazonaws.com`), set these environment variables on the remote host:

```bash
# host.env example for ECR
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
export AWS_DEFAULT_REGION=us-west-2
```

##### Digital Ocean Container Registry Authentication
For Digital Ocean registries (URLs containing `registry.digitalocean.com`), set this environment variable:

```bash
# host.env example for Digital Ocean
export DIGITALOCEAN_ACCESS_TOKEN=dop_v1_your_token_here
```

Then reference the environment file in your `lord.yml`:
```yaml
hostenvironmentfile: host.env
```

### Registry-less Deployment

Lord supports deployment without a container registry by omitting the `registry` field from your configuration. In this mode:

1. The container is built locally using `docker build`
2. The image is saved to a compressed tar.gz file using `docker save`
3. The file is transferred to the remote server via SFTP
4. The image is loaded on the remote server using `docker load`
5. The container is run normally

This approach is useful for:
- Private deployments without registry access
- Simple deployments to single servers
- Development environments
- Air-gapped deployments

Example registry-less configuration:
```yaml
name: myapp
server: 192.168.1.100
hostname: myapp.local
web: true
email: admin@example.com
```

**Note:** Registry-less deployment requires sufficient disk space on both local and remote machines for the compressed container image.

## Installation

### From Source

1. Install Go 1.19+
2. Clone this repository:
   ```sh
   git clone https://github.com/yourusername/lord.git
   cd lord
   ```
3. Build and install:
   ```sh
   make build        # builds ./lord binary
   make install      # installs to /usr/local/bin (requires sudo)
   ```

### Prerequisites

- Docker installed locally for building containers
- SSH key access to your target deployment servers
- Access to a container registry (Docker Hub, GitHub Container Registry, etc.) - optional for registry-less deployment

### Supported Linux Distributions

Lord automatically installs Docker and registry tools on target servers and supports the following Linux distributions:

- **Ubuntu** - Uses apt package manager with Docker's official repository
- **Debian** - Uses apt package manager with Docker's official repository  
- **Amazon Linux 2023** - Uses dnf package manager with Amazon's default repositories
- **CentOS** - Uses yum package manager with Docker's official repository
- **Red Hat Enterprise Linux (RHEL)** - Uses yum package manager with Docker's official repository

Lord automatically detects the host operating system and uses the appropriate package manager and repositories for Docker installation.

## Why Not Use Docker Compose?

Lord takes a different approach from Docker Compose by focusing on unrelated single-container deployments across multiple remote servers. While Docker Compose excels at orchestrating multi-container applications on a single host, Lord is designed for:

- **Simple single-container applications** that don't need complex service orchestration
- **Multi-server deployments** where the same container runs across different hosts
- **Minimal server dependencies** - only Docker is required on the target server
- **Built-in reverse proxy** with automatic TLS certificate management via Traefik
- **Registry-based deployments** where containers are built locally and pushed to registries

Docker Compose requires configuration files on each server and is better suited for applications with multiple interconnected services. Lord eliminates server-side configuration complexity by managing everything through SSH from your local machine.

## License

BSD 3-Clause License - see LICENSE file for details.

