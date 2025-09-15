package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
)

type remote struct {
	address string
	config  *Config
}

func (r *remote) getHostOS() (string, error) {
	var osType string
	var osErr error

	err := withSSHClient(r.address, r.config, func(client *ssh.Client) error {
		// try to identify the distro using /etc/os-release
		osRelease, _, err := runSSHCommand(client, "cat /etc/os-release | grep ^ID= | cut -d'=' -f2 | tr -d '\"'")
		if err == nil && osRelease != "" {
			osType = strings.TrimSpace(osRelease)
			return nil
		}

		// fallback to checking for specific files
		_, _, err = runSSHCommand(client, "test -f /etc/amazon-linux-release")
		if err == nil {
			osType = "amzn"
			return nil
		}

		_, _, err = runSSHCommand(client, "test -f /etc/debian_version")
		if err == nil {
			osType = "debian"
			return nil
		}

		_, _, err = runSSHCommand(client, "test -f /etc/redhat-release")
		if err == nil {
			osType = "rhel"
			return nil
		}

		osType = "unknown"
		osErr = fmt.Errorf("unable to determine host os")
		return nil
	})

	if err != nil {
		return "", err
	}

	return osType, osErr
}

func (r *remote) getDockerInstallCommands(osType string) ([]string, error) {
	switch osType {
	case "ubuntu", "debian":
		return []string{
			"apt-get update",
			"apt-get upgrade -y",
			"for pkg in docker.io docker-doc docker-compose docker-compose-v2 podman-docker containerd runc; do sudo apt-get remove -y $pkg; done",
			"apt-get update",
			"apt-get install -y ca-certificates curl",
			"install -m 0755 -d /etc/apt/keyrings",
			"curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc",
			"chmod a+r /etc/apt/keyrings/docker.asc",
			"echo \"deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo \"$VERSION_CODENAME\") stable\" | /bin/tee /etc/apt/sources.list.d/docker.list > /dev/null",
			"apt-get update",
			"apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin",
			"echo \"{\\\"log-driver\\\": \\\"local\\\"}\" | tee /etc/docker/daemon.json > /dev/null",
			"systemctl enable docker",
			"systemctl restart docker",
			"mkdir -p /root/.docker/",
		}, nil
	case "amzn":
		return []string{
			"dnf update -y",
			"dnf remove -y docker docker-client docker-client-latest docker-common docker-latest docker-latest-logrotate docker-logrotate docker-engine",
			"dnf install -y docker",
			"echo \"{\\\"log-driver\\\": \\\"local\\\"}\" | tee /etc/docker/daemon.json > /dev/null",
			"systemctl enable docker",
			"systemctl restart docker",
			"mkdir -p /root/.docker/",
		}, nil
	case "rhel", "centos":
		return []string{
			"yum update -y",
			"yum remove -y docker docker-client docker-client-latest docker-common docker-latest docker-latest-logrotate docker-logrotate docker-engine",
			"yum install -y yum-utils device-mapper-persistent-data lvm2",
			"yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo",
			"yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin",
			"echo \"{\\\"log-driver\\\": \\\"local\\\"}\" | tee /etc/docker/daemon.json > /dev/null",
			"systemctl enable docker",
			"systemctl restart docker",
			"mkdir -p /root/.docker/",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported os type: %s", osType)
	}
}

func (r *remote) ensureDockerInstalled(authFile string, recover bool) error {
	return withSSHClient(r.address, r.config, func(client *ssh.Client) error {
		if !recover {
			_, _, err := runSSHCommand(client, "docker --version")
			if err == nil {
				return nil
			}
		}

		fmt.Println("installing docker on server")

		// detect host os
		osType, err := r.getHostOS()
		if err != nil {
			return fmt.Errorf("failed to detect host os: %v", err)
		}

		fmt.Printf("detected host os: %s\n", osType)

		// get the appropriate commands for this os
		cmds, err := r.getDockerInstallCommands(osType)
		if err != nil {
			return err
		}

		for _, cmd := range cmds {
			_, _, err := runSSHCommand(client, cmd)
			if err != nil {
				return err
			}
		}

		// only copy auth file if it exists and is specified
		if authFile != "" {
			_, err := os.Stat(authFile)
			if err == nil {
				fmt.Println("copying docker auth file")
				err = sftpCopyFileToRemote(client, authFile, "/root/.docker/config.json")
				if err != nil {
					return err
				}
			} else {
				fmt.Println("no docker auth file provided, skipping registry authentication setup")
			}
		} else {
			fmt.Println("no docker auth file specified, skipping registry authentication setup")
		}

		return nil
	})
}

func (r *remote) ensureDockerRunning() error {
	return withSSHClient(r.address, r.config, func(client *ssh.Client) error {
		_, _, err := runSSHCommand(client, "systemctl is-active --quiet docker")
		if err == nil {
			return nil
		}

		fmt.Println("starting docker on server")

		_, _, err = runSSHCommand(client, "systemctl enable docker")
		if err != nil {
			return fmt.Errorf("could not enable docker on server: %v", err)
		}

		_, _, err = runSSHCommand(client, "systemctl restart docker")
		if err != nil {
			return fmt.Errorf("could not start docker on server: %v", err)
		}

		return nil
	})
}

func (r *remote) pullContainer(imageTag string) error {
	return withSSHClient(r.address, r.config, func(client *ssh.Client) error {
		_, _, err := runSSHCommand(client, fmt.Sprintf("docker pull %s", imageTag))
		if err != nil {
			return err
		}

		return nil
	})
}

func (r *remote) stopAndDeleteContainer(name string) error {
	return withSSHClient(r.address, r.config, func(client *ssh.Client) error {
		fmt.Println("stopping and deleting container if exists")

		_, _, err := runSSHCommand(client, fmt.Sprintf("docker stop | true %s && docker rm --force %s", name, name))
		return err
	})
}

func (r *remote) getContainerStatus(name string) error {
	return withSSHClient(r.address, r.config, func(client *ssh.Client) error {
		fmt.Println("getting container status")

		_, _, err := runSSHCommand(client, fmt.Sprintf("docker ps --filter name=%s", name))
		return err
	})
}

func (r *remote) stageForContainer(name string, volumes []string, environmentFile string) error {
	return withSSHClient(r.address, r.config, func(client *ssh.Client) error {
		fmt.Println("staging host for container")

		cmds := []string{
			fmt.Sprintf("mkdir -p /etc/%s", name),
			fmt.Sprintf("mkdir -p /var/%s", name),
		}

		for _, v := range volumes {
			vParts := strings.Split(v, ":")
			if len(vParts) < 2 {
				return fmt.Errorf("malformed volume mount")
			}

			cmds = append(cmds, fmt.Sprintf("mkdir -p %s", vParts[0]))
		}

		fmt.Println("creating volume mount and config directories")
		for _, cmd := range cmds {
			_, _, err := runSSHCommand(client, cmd)
			if err != nil {
				return err
			}
		}

		if environmentFile != "" {
			fmt.Println("copying env file")
			err := sftpCopyFileToRemote(client, environmentFile, fmt.Sprintf("/etc/%s/%s.env", name, name))
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *remote) runContainer(name string, imageTag string, volumes []string, environmentFile string, web bool, hostname string) error {
	return withSSHClient(r.address, r.config, func(client *ssh.Client) error {
		fmt.Println("running container")

		runCommand := "docker run -d --restart unless-stopped"
		runCommand += fmt.Sprintf(" --name %s", name)
		runCommand += fmt.Sprintf(" -v /var/%s:/data", name)

		for _, volume := range volumes {
			runCommand += fmt.Sprintf(" -v %s", volume)
		}

		if web {
			runCommand += " --label \"traefik.enable=true\""
			runCommand += fmt.Sprintf(" --label \"traefik.http.routers.%s.rule=Host(\\`%s\\`) || Host(\\`www.%s\\`)\"", name, hostname, hostname)
			runCommand += fmt.Sprintf(" --label \"traefik.http.routers.%s.entryPoints=websecure\"", name)
			runCommand += fmt.Sprintf(" --label \"traefik.http.routers.%s.tls.certresolver=theresolver\"", name)
			runCommand += fmt.Sprintf(" --label \"traefik.http.services.%s.loadbalancer.server.port=80\"", name)
			runCommand += " --network traefik"
		}

		if environmentFile != "" {
			runCommand += fmt.Sprintf(" --env-file /etc/%s/%s.env", name, name)
		}

		runCommand += fmt.Sprintf(" %s", imageTag)

		_, _, err := runSSHCommand(client, runCommand)
		if err != nil {
			return err
		}

		return nil
	})
}

func (r *remote) streamContainerLogs(name string) error {
	return withSSHClient(r.address, r.config, func(client *ssh.Client) error {
		fmt.Println("streaming container logs...")

		session, err := client.NewSession()
		if err != nil {
			return err
		}
		defer session.Close()

		stdout, err := session.StdoutPipe()
		if err != nil {
			return err
		}

		stderr, err := session.StderrPipe()
		if err != nil {
			return err
		}

		err = session.Start(fmt.Sprintf("docker logs --follow --tail 30 %s", name))
		if err != nil {
			return err
		}

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		done := make(chan bool, 1)
		go func() {
			<-sigs
			fmt.Println("stopping...")
			session.Signal(ssh.SIGKILL)
			session.Close()
			done <- true
		}()

		go func() {
			_, err := io.Copy(os.Stdout, stdout)
			if err != nil {
				fmt.Printf("error streaming container logs stdout: %v", err)
			}
			done <- true
		}()

		go func() {
			_, err := io.Copy(os.Stderr, stderr)
			if err != nil {
				fmt.Printf("error streaming container logs stderr: %v", err)
			}
			done <- true
		}()

		<-done
		fmt.Println("end log stream")

		return nil
	})
}

func (r *remote) downloadContainerLogs(name string) error {
	return withSSHClient(r.address, r.config, func(client *ssh.Client) error {
		fmt.Println("downloading container log file")

		localLogPath := fmt.Sprintf("./lord-logs/%s-%v.log", name, time.Now().Unix())

		fmt.Printf("downloading logs for container %s to %s\n", name, localLogPath)

		logs, _, err := runSSHCommand(client, fmt.Sprintf("docker logs %s", name))
		if err != nil {
			return err
		}

		return os.WriteFile(localLogPath, []byte(logs), 0644)
	})
}
