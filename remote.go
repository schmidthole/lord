package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/crypto/ssh"
)

type remote struct {
	address string
}

func (r *remote) ensureDockerInstalled(username string, password string) error {
	return withSSHClient(r.address, func(client *ssh.Client) error {
		_, _, err := runSSHCommand(client, "docker --version")
		if err == nil {
			return nil
		}

		fmt.Println("installing docker on server")

		cmds := []string{
			"apt-get update",
			"apt-get upgrade",
			"for pkg in docker.io docker-doc docker-compose docker-compose-v2 podman-docker containerd runc; do sudo apt-get remove $pkg; done",
			"apt-get update",
			"apt-get install ca-certificates curl",
			"install -m 0755 -d /etc/apt/keyrings",
			"curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc",
			"chmod a+r /etc/apt/keyrings/docker.asc",
			"echo \\ \"deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \\ $(. /etc/os-release && echo \"$VERSION_CODENAME\") stable\" | \\ tee /etc/apt/sources.list.d/docker.list > /dev/null",
			"apt-get update",
			"echo \"{\"log-driver\": \"local\"}\" | tee /etc/docker/daemon.json > /dev/null",
			"systemcrl enable docker",
			"systemctl restart docker",
			fmt.Sprintf("docker login registry.digitalocean.com --username %s --password %s", username, password),
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

func (r *remote) ensureDockerRunning() error {
	return withSSHClient(r.address, func(client *ssh.Client) error {
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
	return withSSHClient(r.address, func(client *ssh.Client) error {
		_, _, err := runSSHCommand(client, fmt.Sprintf("docker pull %s", imageTag))
		if err != nil {
			return err
		}

		return nil
	})
}

func (r *remote) stopAndDeleteContainer(name string) error {
	return withSSHClient(r.address, func(client *ssh.Client) error {
		fmt.Println("stopping and deleting container if exists")

		_, _, err := runSSHCommand(client, fmt.Sprintf("docker stop | true %s && docker rm --force %s", name, name))
		return err
	})
}

func (r *remote) getContainerStatus(name string) error {
	return withSSHClient(r.address, func(client *ssh.Client) error {
		fmt.Println("getting container status")

		_, _, err := runSSHCommand(client, fmt.Sprintf("docker ps --filter name=%s", name))
		return err
	})
}

func (r *remote) runContainer(name string, imageTag string) error {
	return withSSHClient(r.address, func(client *ssh.Client) error {
		fmt.Println("running container")

		runCommand := "docker run -d --restart unless-stopped"
		runCommand += fmt.Sprintf(" --name %s", name)
		runCommand += fmt.Sprintf(" -v /var/%s:/data", name)
		runCommand += fmt.Sprintf(" %s", imageTag)

		_, _, err := runSSHCommand(client, runCommand)
		if err != nil {
			return err
		}

		return nil
	})
}

func (r *remote) streamContainerLogs(name string) error {
	return withSSHClient(r.address, func(client *ssh.Client) error {
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
