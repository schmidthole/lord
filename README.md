# Lord

A minimalist PaaS management service for deploying Docker containers to remote hosts.

Lord builds your Docker containers locally and deploys them to Linux servers via SSH. It focuses on simplicity with minimal configuration and zero dependencies on the target server beyond Docker.

## Quick Start

Lord requires three things to get started:

1. A `Dockerfile` in your project directory
2. A container registry (you must be logged in locally)
3. A `lord.yml` configuration file

Get started by running `lord -init` in your project directory to generate a configuration template.

## How It Works

1. **Build**: Lord builds your Docker container locally with the specified platform
2. **Push**: Container is pushed to your configured registry
3. **Deploy**: Lord SSHs to your server and pulls/runs the container
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
```

## Configuration

Create a `lord.yml` file in your project directory:

```yaml
# required fields
name: myapp                           # unique app name per host
registry: my.realregistry.com/me      # container registry url
authfile: ./config.json               # docker registry auth file
server: 192.168.1.100                 # target server ip address

# optional fields
email: user@example.com               # email for tls certificates
platform: linux/amd64                 # build platform (default: linux/amd64)
target: production                    # docker build target stage
web: true                             # enable web service with traefik
hostname: myapp.example.com           # domain name (required if web: true)
environmentfile: .env                 # environment variables file
buildargfile: build.args              # docker build arguments file

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

### Required Registry Setup

You must provide a `config.json` file with registry authentication. This file will be copied to your server to enable container pulls:

```json
{
  "auths": {
    "my.realregistry.com": {
      "auth": "base64encodedcredentials"
    }
  }
}
```

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
- Access to a container registry (Docker Hub, GitHub Container Registry, etc.)

## License

MIT License - see LICENSE file for details.

