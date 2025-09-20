package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

type Registry string

const (
	RegistryUnsupported  Registry = "unsupported"
	RegistryDigitalOcean Registry = "digitalocean"
	RegistryEcr          Registry = "ecr"
)

func detectRegistryType(registryUrl string) Registry {
	if strings.Contains(registryUrl, "amazonaws.com") {
		return RegistryEcr
	}
	if strings.Contains(registryUrl, "digitaloceanspaces.com") || strings.Contains(registryUrl, "registry.digitalocean.com") {
		return RegistryDigitalOcean
	}
	return RegistryUnsupported
}

func (r *remote) getRegistryToolsInstallCommands(registryType Registry, osType string) ([]string, error) {
	switch registryType {
	case RegistryEcr:
		switch osType {
		case "ubuntu", "debian":
			return []string{
				"sudo apt-get update",
				"sudo apt-get install -y awscli",
			}, nil
		case "amzn":
			return []string{}, nil // aws cli is pre-installed on amazon linux
		case "rhel", "centos":
			return []string{
				"sudo yum install -y awscli",
			}, nil
		default:
			return nil, fmt.Errorf("unsupported os type for ecr: %s", osType)
		}
	case RegistryDigitalOcean:
		switch osType {
		case "ubuntu", "debian":
			return []string{
				"sudo apt-get update",
				"sudo apt-get install -y doctl",
			}, nil
		case "amzn", "rhel", "centos":
			return []string{
				"curl -sL https://github.com/digitalocean/doctl/releases/download/v1.95.0/doctl-1.95.0-linux-amd64.tar.gz | sudo tar -xzC /usr/local/bin",
			}, nil
		default:
			return nil, fmt.Errorf("unsupported os type for digitalocean: %s", osType)
		}
	default:
		return nil, fmt.Errorf("unsupported registry type: %s", registryType)
	}
}

func (r *remote) ensureRegistryToolsInstalled(recover bool) error {
	return withSSHClient(r.address, r.config, func(client *ssh.Client) error {
		registryType := detectRegistryType(r.config.Registry)
		if registryType == RegistryUnsupported {
			return fmt.Errorf("unsupported registry, cannot perform install tools")
		}

		if !recover {
			// check if the specific registry tools are already installed and return
			switch registryType {
			case RegistryEcr:
				_, _, err := runSSHCommand(client, "aws --version", "")
				if err == nil {
					return nil
				}
			case RegistryDigitalOcean:
				_, _, err := runSSHCommand(client, "doctl version", "")
				if err == nil {
					return nil
				}
			}
		}

		fmt.Println("installing registry tools on server")

		// detect host os
		osType, err := r.getHostOS()
		if err != nil {
			return fmt.Errorf("failed to detect host os: %v", err)
		}

		fmt.Printf("detected host os: %s\n", osType)

		// get the appropriate commands for this os and registry
		cmds, err := r.getRegistryToolsInstallCommands(registryType, osType)
		if err != nil {
			return err
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

func (r *remote) registryLogin() error {
	return withSSHClient(r.address, r.config, func(client *ssh.Client) error {
		registryType := detectRegistryType(r.config.Registry)
		if registryType == RegistryUnsupported {
			return fmt.Errorf("unsupported registry, cannot perform docker login")
		}

		switch registryType {
		case RegistryEcr:
			fmt.Println("authenticating to ecr registry")
			// get registry region from url
			regionStart := strings.Index(r.config.Registry, ".")
			regionEnd := strings.Index(r.config.Registry[regionStart+1:], ".") + regionStart + 1
			region := r.config.Registry[regionStart+1 : regionEnd]

			// get ecr login token and login to docker
			loginCmd := fmt.Sprintf("aws ecr get-login-password --region %s | sudo docker login --username AWS --password-stdin %s", region, r.config.Registry)
			_, _, err := runSSHCommand(client, loginCmd, "")
			if err != nil {
				return fmt.Errorf("failed to login to ecr: %v", err)
			}

		case RegistryDigitalOcean:
			fmt.Println("authenticating to digitalocean registry")
			// use doctl to authenticate and login to docker
			loginCmd := "doctl registry login"
			_, _, err := runSSHCommand(client, loginCmd, "")
			if err != nil {
				return fmt.Errorf("failed to login to digitalocean registry: %v", err)
			}

		default:
			return fmt.Errorf("unsupported registry type: %s", registryType)
		}

		return nil
	})
}

func (r *remote) ensureRegistryAuthenticated(recover bool) error {
	if r.config.Registry == "" {
		fmt.Println("no container registry in use, skipping authentication")
		return nil
	}

	return withSSHClient(r.address, r.config, func(client *ssh.Client) error {
		// only copy auth file if it exists and is specified
		if r.config.AuthFile != "" {
			_, err := os.Stat(r.config.AuthFile)
			if err == nil {
				fmt.Println("copying docker auth file")
				err = sftpCopyFileToRemote(client, r.config.AuthFile, "/root/.docker/config.json")
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("no docker registry auth file found at: %s", r.config.AuthFile)
			}
		} else {
			fmt.Println("no docker registry auth file specified, attempting docker login")

			err := r.ensureRegistryToolsInstalled(recover)
			if err != nil {
				return err
			}

			err = r.registryLogin()
			if err != nil {
				return err
			}
		}

		return nil
	})
}
