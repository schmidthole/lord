package main

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

var traefikConfigTemplate = `
entryPoints:
  web:
    address: ":80"
  websecure:
    address: ":443"

certificatesResolvers:
  theresolver:
    acme:
      email: %v
      storage: acme.json
      httpChallenge:
        entryPoint: web

providers:
  docker:
    exposedByDefault: false
`

func (r *remote) ensureTraefikSetup(email string) error {
	return withSSHClient(r.address, r.config, func(client *ssh.Client) error {
		stdOut, _, err := runSSHCommand(client, "docker ps --filter name=traefik --format \"{{.Names}}\"")
		if err != nil {
			return err
		}

		if strings.Contains(stdOut, "traefik") {
			fmt.Println("traefik already running...")
			return nil
		}

		traefikConfig := fmt.Sprintf(traefikConfigTemplate, email)

		fmt.Println("checking for traefik docker network")

		stdOut, _, err = runSSHCommand(client, "docker network ls --format '{{.Name}}'")
		if err != nil {
			return err
		}

		if !strings.Contains(stdOut, "traefik") {
			_, _, err = runSSHCommand(client, "docker network create traefik")
		}
		if err != nil {
			return err
		}

		fmt.Println("setting up traefik on server")

		cmds := []string{
			"mkdir -p /etc/traefik",
			fmt.Sprintf("cat > /etc/traefik/traefik.yml <<EOF\n %v \nEOF", traefikConfig),
			"touch /etc/traefik/acme.json",
			"chmod 600 /etc/traefik/acme.json",
			"docker rm --force traefik",
			"docker run -d --restart unless-stopped --name traefik -v /var/run/docker.sock:/var/run/docker.sock -v /etc/traefik/traefik.yml:/etc/traefik/traefik.yml -v /etc/traefik/acme.json:/acme.json -p 80:80 -p 443:443 --network traefik traefik:latest",
		}

		for _, cmd := range cmds {
			_, _, err := runSSHCommand(client, cmd)
			if err != nil {
				return err
			}
		}

		return nil
	})
}
