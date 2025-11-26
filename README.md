# LORD

```
██╗      ██████╗ ██████╗ ██████╗ 
██║     ██╔═══██╗██╔══██╗██╔══██╗
██║     ██║   ██║██████╔╝██║  ██║
██║     ██║   ██║██╔══██╗██║  ██║
███████╗╚██████╔╝██║  ██║██████╔╝
╚══════╝ ╚═════╝ ╚═╝  ╚═╝╚═════╝ 
```

An ultra minimalist PaaS management service for deploying Docker containers. All you need is Docker and SSH access to a server. Inspired by other "self hosting" PaaS frameworks and the need to break free from the complexity of "cloud native" services for the majority of real world applications.

Lord utilizes existing container utilities and your local machine (or CI/CD provider) as the "builder" for your applications, automating container deployment/hosting with the following core features:

* Automatic configuration of remote host for container deployment
* Seamless deploy and destroy of application
* Direct container load or push/pull via registry
* Supports hosting several unrelated web apps and containers on a single host
* Automatically serves web apps via https (using Traefik reverse proxy)
* Log tailing and system monitoring via CLI or Dozzle UI

Deploy from your PC or github actions. Deploy to a remote EC2 instance or a Linux server in your bedroom. Lord doesn't care.

# Installation

**Prerequisites**

- Docker installed locally
- SSH key access to your target deployment server(s)

**Binary Releases (Recommended)**

```bash
# macOS (Apple Silicon)
curl -fsSL https://github.com/schmidthole/lord/releases/latest/download/lord-macos-arm64 -o /tmp/lord && sudo mv /tmp/lord /usr/local/bin/lord && sudo chmod +x /usr/local/bin/lord

# macOS (Intel)
curl -fsSL https://github.com/schmidthole/lord/releases/latest/download/lord-macos-amd64 -o /tmp/lord && sudo mv /tmp/lord /usr/local/bin/lord && sudo chmod +x /usr/local/bin/lord

# Linux (x86_64)
curl -fsSL https://github.com/schmidthole/lord/releases/latest/download/lord-linux-amd64 -o /tmp/lord && sudo mv /tmp/lord /usr/local/bin/lord && sudo chmod +x /usr/local/bin/lord
```

**From Source**

1. Install Go 1.19+
2. Clone this repository:
   ```sh
   git clone https://github.com/schmidthole/lord.git
   cd lord
   ```
3. Build and install:
   ```sh
   make build        # builds ./lord binary
   make install      # installs to /usr/local/bin (requires sudo)
   ```

# Commands

All lord commands are run from your project root directory and require a `lord.yml` configuration file to be present.

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

# How Does it Work

Lord is meant to be as simple to setup and run as possible. To get started you need:

* A project/application with a `Dockerfile`
* SSH key access to a server
* DNS A records pointing to your server if hosting a web application

Initialize Lord by running `lord -init` in your project root directory. This will create a base `lord.yml` configuration file which tells Lord how to deploy your application.

Edit the newly created `lord.yml` file:

* `name` will contain your applications unique name
* `server` will contain your server's IP address

If hosting a web application, set the following fields to automatically host your application via https:

* Set `web` to `true`
* Place your applications domain/hostname in `hostname`

Run `lord -deploy` to configure your server and deploy your container. Once deployed, your web application should be accessible via your custom domain if applicable.

After deployment, run `lord -destroy` to stop/remove your application at any time.

You may also wish to monitor your application with the following commands:

* `lord -status` to get basic status
* `lord -logs` to tail application logs in realtime
* `lord -monitor` to check system load of the host
* `lord -dozzle` to connect and run the dozzle container monitoring UI, which will allow you to view all containers on the host

## Container Conventions

Lord requires the following minimal set of conventions for all containers it deploys:

* Web services must expose port `80` internally
* Persistent data should use the `/data` volume mount
* Additional volumes can be specified in configuration

# Configuration

The following is a complete reference to Lord's configuration values:

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
environmentfile: .env                 # container environment variables file
buildargfile: build.args              # docker build arguments file
hostenvironmentfile: host.env         # host environment variables file
user: ubuntu                          # ssh login user (default: root)
sshkeyfile: /path/to/private/key      # custom ssh private key file

# additional volume mounts (follows docker format)
volumes:
  - /host/data:/container/data
  - /etc/config:/app/config

# advanced web configuration (optional)
webadvancedconfig:
  # timeout settings (affects global traefik reverse proxy - use with caution!)
  readtimeout: 300                    # maximum duration in seconds for reading the entire request (0 = unlimited)
  writetimeout: 300                   # maximum duration in seconds before timing out writes of the response (0 = unlimited)
  idletimeout: 180                    # maximum duration in seconds an idle connection is kept alive

  # buffering settings (per-service, safe to use)
  maxrequestbodybytes: 10485760       # maximum allowed size in bytes of the request body (10MB)
  maxresponsebodybytes: 10485760      # maximum allowed size in bytes of the response body (10MB)
  memrequestbodybytes: 1048576        # threshold in bytes after which request body is buffered to disk (1MB)
```

## Advanced Web Configuration

Lord supports advanced Traefik configuration for handling large file uploads/downloads and long-running requests. This is particularly useful for applications like container registries, file upload services, or long-polling APIs.

### ⚠️ Timeout Settings (Use With Caution)

The timeout settings (`readtimeout`, `writetimeout`, `idletimeout`) configure the **global Traefik reverse proxy** that serves all web containers on the host. These settings affect **every web application** deployed via Lord on the same server.

**Important notes:**
- Timeout values are specified in **seconds**
- Setting a timeout to `0` means **unlimited** (no timeout)
- Lord uses a "maximum wins" strategy: if multiple deployments specify different timeout values, the highest value is used globally
- Once a timeout is increased, it will remain at that value unless manually changed in the Traefik config
- Higher timeouts may increase vulnerability to slowloris attacks and resource exhaustion

**When to use timeout settings:**
- Hosting a Docker registry that needs to handle large image uploads
- File upload services that may take several minutes
- Long-polling or streaming APIs
- Applications with slow clients or large request/response bodies

**Example for a container registry:**
```yaml
webadvancedconfig:
  readtimeout: 0      # unlimited - allow large image uploads
  writetimeout: 0     # unlimited - allow large image downloads
  idletimeout: 300    # 5 minutes - disconnect idle connections
```

### Buffering Settings (Per-Service)

The buffering settings configure request/response body size limits on a **per-service basis** and are safe to use without affecting other applications.

- `maxrequestbodybytes`: Maximum size of incoming request body (prevents oversized uploads)
- `maxresponsebodybytes`: Maximum size of outgoing response body (prevents oversized downloads)
- `memrequestbodybytes`: Size threshold before buffering to disk instead of memory

**Example for large file uploads:**
```yaml
webadvancedconfig:
  maxrequestbodybytes: 5368709120    # 5GB max upload
  maxresponsebodybytes: 5368709120   # 5GB max download
  memrequestbodybytes: 10485760      # buffer to disk after 10MB
```

# Supported Linux Distributions

Lord automatically installs Docker and registry tools on target servers and supports the following Linux distributions:

- **Ubuntu** - Uses apt package manager with Docker's official repository
- **Debian** - Uses apt package manager with Docker's official repository  
- **Amazon Linux 2023** - Uses dnf package manager with Amazon's default repositories
- **CentOS** - Uses yum package manager with Docker's official repository
- **Red Hat Enterprise Linux (RHEL)** - Uses yum package manager with Docker's official repository

Lord automatically detects the host operating system and uses the appropriate package manager and repositories for Docker installation.

# Advanced Usage

## Supporting Multiple Applications/Containers

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

To perform actions against each separate config, the `-config` flag can be included in the Lord command along with the config key:

``` sh
lord -config conf2 -deploy
```

## Environment Variables

### Remote Server Environment Variables

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

### Container Environment Variables

Environment variables can be injected into the container via a `.env` file supplied in the `lord.yml` file. The environment variables contained in this file are only available to the container during runtime and not to the remote host during deployment.

## Registry Usage

Lord optionally supports the ability to push/pull a container via a supported registry provided instead of direct save/transfer/load onto the remote host. This doesn't pose much advantage currently, but registries will become a more useful in the future once Lord supports multiple load balanced hosts, rollbacks, etc.

## Registry Authentication

Lord supports three methods for registry authentication:

### Method 1: Username/Password File (Simplest)
Provide a plain text file containing your registry username and password separated by a colon:

```
myusername:mypassword
```

Lord will automatically detect this format and perform a `docker login` on the remote host using these credentials. This is the simplest method for generic container registries.

Example `lord.yml` configuration:
```yaml
registry: my.realregistry.com/me
authfile: ./registry-auth.txt
```

### Method 2: Docker Config File (Advanced)
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

Lord will automatically detect JSON format and copy this file to the remote host's `.docker/config.json`.

**NOTE:** this method overwrites the entire `.docker/config.json` file on the host. In order to use this
method, all containers on the host must share the same registry and authentication method.

### Method 3: Dynamic Authentication (Cloud Providers)
Lord can automatically authenticate to supported registries using environment variables. Currently supported:

- **AWS ECR** - requires AWS credentials
- **Digital Ocean Container Registry** - requires Digital Ocean API token

When no `authfile` is specified in `lord.yml`, Lord will attempt dynamic authentication based on the registry URL.

#### AWS ECR Authentication
For ECR registries (URLs containing `amazonaws.com`), set these environment variables on the remote host:

```bash
# host.env example for ECR
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
export AWS_DEFAULT_REGION=us-west-2
```

#### Digital Ocean Container Registry Authentication
For Digital Ocean registries (URLs containing `registry.digitalocean.com`), set this environment variable:

```bash
# host.env example for Digital Ocean
export DIGITALOCEAN_ACCESS_TOKEN=dop_v1_your_token_here
```

Then reference the environment file in your `lord.yml`:
```yaml
hostenvironmentfile: host.env
```

# Why Not Use Docker Compose?

Lord takes a different approach from Docker Compose by focusing on unrelated single-container deployments across multiple remote servers. While Docker Compose excels at orchestrating multi-container applications on a single host, Lord is designed for:

- **Simple single-container applications** that don't need complex service orchestration
- **Multi-server deployments** where the same container runs across different hosts
- **Minimal server dependencies** - only Docker is required on the target server
- **Built-in reverse proxy** with automatic TLS certificate management via Traefik
- **Registry-based deployments** where containers are built locally and pushed to registries

Docker Compose requires configuration files on each server and is better suited for applications with multiple interconnected services. Lord eliminates server-side configuration complexity by managing everything through SSH from your local machine.

# Roadmap

Lord is very simple and focused, but could get more features in the future. These are a few that are being worked on:

* Rollbacks via registry
* Load balanced traffic to multiple remote hosts
* Automated deployment and connection to self-hosted container registry

# License

BSD 3-Clause License - see LICENSE file for details.
