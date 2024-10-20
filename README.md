# Lord

This is a very opinionated an minimalist PaaS management service. `lord` will build a docker
container for a given project and deploy it to a linux host. The goal is to have as few
configuration options and dependencies as possible.

`lord` doesn't care what is running on the host outside of the details specified for the current
project. This means a bunch of stuff can be running already.

**In order for lord to function it needs:**

1. A `Dockerfile` in the current directory to build.
2. A registry to push and pull docker images from.
3. A `lord.yml` file in the current directory alongside the `Dockerfile`

Simply run `lord` in the current directory (assuming the binary is in your path) and the following will happen:

1. Your docker container will be built.
2. The container will be pushed to the specified registry.
3. `lord` will use ssh to:
  1. Ensure docker is running and installed on the specified server.
  2. Ensure your server is logged into the specified registry.
  3. Pull the container from the registry onto the server.
  4. Run the container on the server using the specified options.

## Lord Config File Format

Your project's `lord.yml` should look like this:

```yaml
# image name. this will appear in the container names and tags
name: test

# container registry
registry: my.real.registry.com/me

# registry username and password
username: theuser
password: abcdefghijklmnopqrstuvwxyz

# the server to deploy to. lord will use the root user and ssh key authentication by default
server: 161.35.141.177

# an optional list of persistent volumes your container requires
volumes:
 - /etc/test/data:/data

# an optional builder platform for the docker container. this will default to linux/amd64
platform: linux/amd64
```

## Installation

1. Install Go
2. Clone this repo and run:

```sh
make build
```

3. Put the `lord` binary somewhere in your `$PATH`. 

*Better install script/instructions to come in the future.*

## Roadmap

* [ ] Exposing web services via ports.
* [ ] Auto configuration of a reverse proxy with ssl certs.
* [ ] Log streaming/viewing.