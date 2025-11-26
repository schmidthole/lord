package main

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

type TraefikConfig struct {
	EntryPoints           map[string]EntryPoint          `yaml:"entryPoints"`
	CertificatesResolvers map[string]CertificateResolver `yaml:"certificatesResolvers"`
	Providers             Providers                      `yaml:"providers"`
}

type EntryPoint struct {
	Address   string               `yaml:"address"`
	Transport *EntryPointTransport `yaml:"transport,omitempty"`
}

type EntryPointTransport struct {
	RespondingTimeouts *RespondingTimeouts `yaml:"respondingTimeouts,omitempty"`
}

type RespondingTimeouts struct {
	ReadTimeout  string `yaml:"readTimeout,omitempty"`
	WriteTimeout string `yaml:"writeTimeout,omitempty"`
	IdleTimeout  string `yaml:"idleTimeout,omitempty"`
}

type CertificateResolver struct {
	ACME ACMEConfig `yaml:"acme"`
}

type ACMEConfig struct {
	Email         string        `yaml:"email"`
	Storage       string        `yaml:"storage"`
	HTTPChallenge HTTPChallenge `yaml:"httpChallenge"`
}

type HTTPChallenge struct {
	EntryPoint string `yaml:"entryPoint"`
}

type Providers struct {
	Docker DockerProvider `yaml:"docker"`
}

type DockerProvider struct {
	ExposedByDefault bool `yaml:"exposedByDefault"`
}

func (tc *TraefikConfig) serialize() (string, error) {
	yamlBytes, err := yaml.Marshal(&tc)
	if err != nil {
		return "", err
	}

	return string(yamlBytes), nil
}

func createTraefikConfig(email string, webAdvancedConfig WebAdvancedConfig) (string, error) {
	config := TraefikConfig{
		EntryPoints: map[string]EntryPoint{
			"web": {
				Address: ":80",
			},
			"websecure": {
				Address: ":443",
			},
		},
		CertificatesResolvers: map[string]CertificateResolver{
			"theresolver": {
				ACME: ACMEConfig{
					Email:   email,
					Storage: "acme.json",
					HTTPChallenge: HTTPChallenge{
						EntryPoint: "web",
					},
				},
			},
		},
		Providers: Providers{
			Docker: DockerProvider{
				ExposedByDefault: false,
			},
		},
	}

	// add responding timeouts if any are set
	if webAdvancedConfig.ReadTimeout != -1 || webAdvancedConfig.WriteTimeout != -1 || webAdvancedConfig.IdleTimeout != -1 {
		timeouts := &RespondingTimeouts{}

		if webAdvancedConfig.ReadTimeout != -1 {
			timeouts.ReadTimeout = fmt.Sprintf("%ds", webAdvancedConfig.ReadTimeout)
		}
		if webAdvancedConfig.WriteTimeout != -1 {
			timeouts.WriteTimeout = fmt.Sprintf("%ds", webAdvancedConfig.WriteTimeout)
		}
		if webAdvancedConfig.IdleTimeout != -1 {
			timeouts.IdleTimeout = fmt.Sprintf("%ds", webAdvancedConfig.IdleTimeout)
		}

		websecure := config.EntryPoints["websecure"]
		websecure.Transport = &EntryPointTransport{
			RespondingTimeouts: timeouts,
		}
		config.EntryPoints["websecure"] = websecure
	}

	return config.serialize()
}

func readTraefikConfig(yamlString string) (*TraefikConfig, error) {
	var config TraefikConfig
	err := yaml.Unmarshal([]byte(yamlString), &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func maybeUpdateTraefikAdvancedWebConfig(config *TraefikConfig, webAdvancedConfig WebAdvancedConfig) bool {
	updated := false

	// ensure websecure entrypoint exists
	websecure, exists := config.EntryPoints["websecure"]
	if !exists {
		return false
	}

	// ensure transport exists
	if websecure.Transport == nil {
		websecure.Transport = &EntryPointTransport{}
	}

	// ensure responding timeouts exists
	if websecure.Transport.RespondingTimeouts == nil {
		websecure.Transport.RespondingTimeouts = &RespondingTimeouts{}
	}

	timeouts := websecure.Transport.RespondingTimeouts

	// helper function to parse timeout strings like "60s" to int
	parseTimeout := func(s string) int {
		if s == "" {
			return -1
		}
		if s == "0s" || s == "0" {
			return 0
		}
		var val int
		fmt.Sscanf(s, "%ds", &val)
		return val
	}

	// helper function to compare and update timeout (0 = unlimited, higher values win)
	updateIfGreater := func(current string, newValue int) (string, bool) {
		if newValue == -1 {
			// not set in new config, keep current
			return current, false
		}

		currentVal := parseTimeout(current)

		// if current is 0 (unlimited), keep it
		if currentVal == 0 {
			return current, false
		}

		// if new value is 0 (unlimited) or greater than current, update
		if newValue == 0 || (currentVal != -1 && newValue > currentVal) || currentVal == -1 {
			return fmt.Sprintf("%ds", newValue), true
		}

		return current, false
	}

	// update read timeout
	if newVal, changed := updateIfGreater(timeouts.ReadTimeout, webAdvancedConfig.ReadTimeout); changed {
		timeouts.ReadTimeout = newVal
		updated = true
	}

	// update write timeout
	if newVal, changed := updateIfGreater(timeouts.WriteTimeout, webAdvancedConfig.WriteTimeout); changed {
		timeouts.WriteTimeout = newVal
		updated = true
	}

	// update idle timeout
	if newVal, changed := updateIfGreater(timeouts.IdleTimeout, webAdvancedConfig.IdleTimeout); changed {
		timeouts.IdleTimeout = newVal
		updated = true
	}

	// write back the updated values
	websecure.Transport.RespondingTimeouts = timeouts
	config.EntryPoints["websecure"] = websecure

	return updated
}

func (r *remote) traefikNeedsAdvancedConfig() bool {
	return (r.config.WebAdvancedConfig.ReadTimeout != -1) ||
		(r.config.WebAdvancedConfig.WriteTimeout != -1) ||
		(r.config.WebAdvancedConfig.IdleTimeout != -1)
}

func (r *remote) ensureTraefikSetup(email string) error {
	return withSSHClient(r.address, r.config, func(client *ssh.Client) error {
		stdOut, _, err := runSSHCommand(client, "sudo docker ps --filter name=traefik --format \"{{.Names}}\"", "")
		if err != nil {
			return err
		}

		if strings.Contains(stdOut, "traefik") {
			fmt.Println("traefik already running...")

			if r.traefikNeedsAdvancedConfig() {
				currentTraefikConfigRaw, _, err := runSSHCommand(client, "sudo cat /etc/traefik/traefik.yml", "")
				if err != nil {
					return fmt.Errorf("error reading traefik config: %s", err)
				}

				traefikConfig, err := readTraefikConfig(currentTraefikConfigRaw)
				if err != nil {
					return fmt.Errorf("error parsing traefik config to check for advanced config: %s", err)
				}

				updated := maybeUpdateTraefikAdvancedWebConfig(traefikConfig, r.config.WebAdvancedConfig)
				if updated {
					fmt.Println("updating traefik timeout configuration")

					newTraefikConfig, err := traefikConfig.serialize()
					if err != nil {
						return fmt.Errorf("error serializing new traefik config: %s", err)
					}

					cmds := []string{
						fmt.Sprintf("sudo cat > /etc/traefik/traefik.yml <<EOF\n%v\nEOF", newTraefikConfig),
						"sudo docker restart traefik",
					}

					for _, cmd := range cmds {
						_, _, err := runSSHCommand(client, cmd, "")
						if err != nil {
							return err
						}
					}

					fmt.Println("traefik configuration updated and restarted")
				}
			}

			return nil
		}

		traefikConfig, err := createTraefikConfig(email, r.config.WebAdvancedConfig)
		if err != nil {
			return fmt.Errorf("error creating traefik config: %s", err)
		}

		fmt.Println("checking for traefik docker network")

		stdOut, _, err = runSSHCommand(client, "sudo docker network ls --format '{{.Name}}'", "")
		if err != nil {
			return err
		}

		if !strings.Contains(stdOut, "traefik") {
			_, _, err = runSSHCommand(client, "sudo docker network create traefik", "")
		}
		if err != nil {
			return err
		}

		fmt.Println("setting up traefik on server")

		cmds := []string{
			"sudo mkdir -p /etc/traefik",
			fmt.Sprintf("sudo cat > /etc/traefik/traefik.yml <<EOF\n %v \nEOF", traefikConfig),
			"sudo touch /etc/traefik/acme.json",
			"sudo chmod 600 /etc/traefik/acme.json",
			"sudo docker rm --force traefik",
			"sudo docker run -d --restart unless-stopped --name traefik -v /var/run/docker.sock:/var/run/docker.sock -v /etc/traefik/traefik.yml:/etc/traefik/traefik.yml -v /etc/traefik/acme.json:/acme.json -p 80:80 -p 443:443 --network traefik traefik:latest",
		}

		for _, cmd := range cmds {
			_, _, err := runSSHCommand(client, cmd, "")
			if err != nil {
				return err
			}
		}

		return nil
	})
}
