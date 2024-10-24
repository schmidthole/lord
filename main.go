package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
)

var banner = `

     ___       ___           ___           ___     
    /\__\     /\  \         /\  \         /\  \    
   /:/  /    /::\  \       /::\  \       /::\  \   
  /:/  /    /:/\:\  \     /:/\:\  \     /:/\:\  \  
 /:/  /    /:/  \:\  \   /::\~\:\  \   /:/  \:\__\ 
/:/__/    /:/__/ \:\__\ /:/\:\ \:\__\ /:/__/ \:|__|
\:\  \    \:\  \ /:/  / \/_|::\/:/  / \:\  \ /:/  /
 \:\  \    \:\  /:/  /     |:|::/  /   \:\  /:/  / 
  \:\  \    \:\/:/  /      |:|\/__/     \:\/:/  /  
   \:\__\    \::/  /       |:|  |        \::/__/   
    \/__/     \/__/         \|__|         ~~       

`

var baseConfig = `name: myapp
registry: my.realregistry.com/me
username: user
password: password
server: 0.0.0.0
volumes:
 - /etc/test/data:/data
`

type config struct {
	Name     string
	Registry string
	Username string
	Password string
	Server   string
	Volumes  []string
	Platform string
}

func loadConfig() (*config, error) {
	viper.SetConfigName("lord")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.SetDefault("platform", "linux/amd64")

	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	var c config
	err = viper.Unmarshal(&c)
	if err != nil {
		return nil, err
	}

	fmt.Println("config loaded")

	return &c, nil
}

func runCommand(command string, args ...string) (string, string, error) {
	cmd := exec.Command(command, args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	fmt.Printf("> %s\n", strings.Join(append([]string{command}, args...), " "))

	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		fmt.Println(stderrBuf.String())
	}

	return stdoutBuf.String(), stderrBuf.String(), err

}

func buildAndPushContainer(imageName string, tag string, platform string) error {
	fmt.Println("building container")

	_, _, err := runCommand("docker", "build", "--platform", platform, "-t", imageName, ".")
	if err != nil {
		return err
	}

	_, _, err = runCommand("docker", "tag", imageName, tag)
	if err != nil {
		return err
	}

	fmt.Println("pushing container to registry")

	_, _, err = runCommand("docker", "push", tag)
	if err != nil {
		return err
	}

	return nil
}

func getSSHClient(server string) (*ssh.Client, error) {
	authMethod, err := getAuthMethod()
	if err != nil {
		panic(err)
	}

	sshConfig := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	return ssh.Dial("tcp", fmt.Sprintf("%s:22", server), sshConfig)
}

func getAuthMethod() (ssh.AuthMethod, error) {
	defaultKeyPaths := []string{
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519"),
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa"),
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_dsa"),
	}

	sshKeyPath := ""

	for _, keyPath := range defaultKeyPaths {
		_, err := os.Stat(keyPath)
		if err == nil {
			sshKeyPath = keyPath
		}
	}

	if sshKeyPath == "" {
		return nil, fmt.Errorf("no default ssh key found")
	}

	fmt.Printf("using ssh key: %v\n", sshKeyPath)

	key, err := os.ReadFile(sshKeyPath)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	return ssh.PublicKeys(signer), nil
}

func runSSHCommand(client *ssh.Client, cmd string) (string, string, error) {
	session, err := client.NewSession()
	if err != nil {
		panic(err)
	}
	defer session.Close()

	fmt.Printf("> %s\n", cmd)

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	err = session.Run(cmd)
	if err != nil {
		fmt.Println(stderrBuf.String())
		return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("command execution failed: %v", err)
	}

	fmt.Println(stdoutBuf.String())

	return stdoutBuf.String(), stderrBuf.String(), nil
}

func ensureDockerInstalled(client *ssh.Client, username string, password string) error {
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
}

func ensureDockerRunning(client *ssh.Client) error {
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
}

func pullContainer(client *ssh.Client, imageTag string) error {
	_, _, err := runSSHCommand(client, fmt.Sprintf("docker pull %s", imageTag))
	if err != nil {
		return err
	}

	return nil
}

func stopAndDeleteContainer(client *ssh.Client, name string) error {
	fmt.Println("stopping and deleting container if exists")

	_, _, err := runSSHCommand(client, fmt.Sprintf("docker stop | true %s && docker rm --force %s", name, name))
	return err
}

func runContainer(client *ssh.Client, name string, imageTag string, volumes []string) error {
	fmt.Println("running container")

	runCommand := "docker run -d --restart unless-stopped"
	runCommand += fmt.Sprintf(" --name %s", name)
	for _, volume := range volumes {
		runCommand += fmt.Sprintf(" -v %s", volume)
	}
	runCommand += fmt.Sprintf(" %s", imageTag)

	_, _, err := runSSHCommand(client, runCommand)
	if err != nil {
		return err
	}

	return nil
}

func streamContainerLogs(client *ssh.Client, name string) error {
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

	err = session.Start(fmt.Sprintf("docker logs --follow %s", name))
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
}

func main() {
	fmt.Println(banner)

	deployFlag := flag.Bool("deploy", false, "build and deploy the container")
	logsFlag := flag.Bool("logs", false, "get logs from the running container")
	initFlag := flag.Bool("init", false, "initialize lord config in current directory")
	destroyFLag := flag.Bool("destroy", false, "stop and delete a running container")

	flag.Parse()

	if *initFlag {
		_, err := os.Stat("lord.yml")
		if err == nil {
			fmt.Println("lord already initialized in current directory")
			return
		} else if os.IsNotExist(err) {
			err := os.WriteFile("lord.yml", []byte(baseConfig), 0644)
			if err != nil {
				panic(err)
			} else {
				fmt.Println("lord initialized successfully in current directory")
			}
		} else {
			panic(err)
		}

		return
	}

	c, err := loadConfig()
	if err != nil {
		panic(err)
	}

	if *deployFlag {
		imageTag := fmt.Sprintf("%s/%s:latest", c.Registry, c.Name)

		err = buildAndPushContainer(c.Name, imageTag, c.Platform)
		if err != nil {
			panic(err)
		}

		fmt.Printf("connecting server: %s\n", c.Server)

		client, err := getSSHClient(c.Server)
		if err != nil {
			panic(err)
		}
		defer client.Close()

		fmt.Println("checking server state")

		err = ensureDockerInstalled(client, c.Username, c.Password)
		if err != nil {
			panic(err)
		}

		err = ensureDockerRunning(client)
		if err != nil {
			panic(err)
		}

		fmt.Println("updating and running container on server")

		err = pullContainer(client, imageTag)
		if err != nil {
			panic(err)
		}

		err = stopAndDeleteContainer(client, c.Name)
		if err != nil {
			panic(err)
		}

		err = runContainer(client, c.Name, imageTag, c.Volumes)
		if err != nil {
			panic(err)
		}

		fmt.Println("finished deployment")
	} else if *logsFlag {
		fmt.Printf("connecting server: %s\n", c.Server)

		client, err := getSSHClient(c.Server)
		if err != nil {
			panic(err)
		}
		defer client.Close()

		err = streamContainerLogs(client, c.Name)
		if err != nil {
			panic(err)
		}
	} else if *destroyFLag {
		fmt.Printf("connecting server: %s\n", c.Server)

		client, err := getSSHClient(c.Server)
		if err != nil {
			panic(err)
		}
		defer client.Close()

		err = stopAndDeleteContainer(client, c.Name)
		if err != nil {
			panic(err)
		}
	} else {
		fmt.Println("No command specified\n\nUsage:")
		flag.VisitAll(func(f *flag.Flag) {
			fmt.Printf("-%s: %s\n", f.Name, f.Usage)
		})
	}
}
