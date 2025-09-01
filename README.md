# Lord

A very opinionated and minimalist PaaS management service. 

`lord` will build a docker container for a given project and deploy it to a linux host. 
The goal is to have as few configuration options and dependencies as possible.

`lord` doesn't care what is running on the host outside of the details specified for the current
project and configuration, 

**In order for lord to function it needs:**

1. A `Dockerfile` in the current directory to build.
2. A registry to push and pull docker images from.
   * You must be logged into your registry locally. `lord` will not do this for you.
   * Part of the configuration of `lord` requires that you provide a `config.json` file with at least read-only auth credentials to your registry. This will be placed on the server.
3. A `lord.yml` file in the current directory alongside the `Dockerfile`

## Container Requirements

To keep things ultra simple, lord requires the following from any containers deployed:

* If the container hosts a web service, it must expose this on its internal port `80`.
* Lord provides a volume mount internal to the container at `/data`. If the container needs to store any persistent data. The `Dockerfile` will need to declare this as well.
* Additional volumes can be specified in the `lord.yml`, these will also need to be exposed in the `Dockerfile`.

## Usage

*Assuming the `lord` binary is in your `$PATH`*

Run `lord -init` to create a base `lord.yml` config file in your current directory.

Run `lord -deploy` and the following will happen:

1. Your docker container described by the `Dockerfile` in your current directory will be built.
2. The container will be pushed to the specified registry.
3. `lord` will use ssh to:
  1. Ensure docker is running and installed on the specified server.
  2. Ensure your server is logged into the specified registry.
  3. Pull the container from the registry onto the server.
  4. Run the container on the server using the specified options.

Run `lord -logs` to stream container logs from your server.

Run `lord -destroy` to remove any running containers from your server associated with the config in the
current directory.

## Lord Config File Format

Your project's `lord.yml` should look like this:

```yaml
# image name. this will appear in the container names and tags
name: test

# container registry
registry: my.real.registry.com/me

# email for tls certs
email: theuser@example.com

# docker auth file to place on the server
authfile: path/to/config.json

# the server to deploy to. lord will use the root user and ssh key authentication by default
server: 161.35.141.177

# an optional builder platform for the docker container. this will default to linux/amd64
platform: linux/amd64

# optional volumes to mount in addition to the default. these follow the standard docker cli format for volumes.
volumes:
    - /etc/app:/config
```

## Installation

1. Install Go
2. Clone this repo and run:

```sh
make build
```

3. Put the `lord` binary somewhere in your `$PATH`, or run:

``` sh
make install
```

to put it in `/usr/local/bin` (requires sudo).

*Better install script/instructions to come in the future.*

