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
	return withSSHClient(r.address, func(client *ssh.Client) error {
		stdOut, _, err := runSSHCommand(client, "docker ps --filter name=traefik")
		if err != nil {
			return err
		}

		lines := strings.Split(strings.TrimSpace(stdOut), "\n")
		if len(lines) >= 1 && lines[0] != "" {
			fmt.Println("traefik already running...")
			return nil
		}

		fmt.Println("setting up traefik on server")

		traefikConfig := fmt.Sprintf(traefikConfigTemplate, email)

		cmds := []string{
			"mkdir -p /etc/traefik",
			fmt.Sprintf("cat > /etc/traefik/traefik.yml <<EOF\n %v \nEOF", traefikConfig),
			"touch /etc/traefik/acme.json",
			"chmod 600 /etc/traefik/acme.json",
			"docker network create traefik",
			"docker run -d --name traefik -v /var/run/docker.sock:/var/run/docker.sock -v /etc/traefik/traefik.yml:/etc/traefik/traefik.yml -v /etc/traefik/acme.json:/acme.json -p 80:80 -p 443:443 --network traefik traefik:latest",
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
