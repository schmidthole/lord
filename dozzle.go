package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"runtime"
	"time"

	"golang.org/x/crypto/ssh"
)

func startDozzleUI(server string, config *Config) error {
	fmt.Println("starting dozzle ui")

	// get ssh client
	client, err := getSSHClient(server, config)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %v", err)
	}
	defer client.Close()

	// setup ssh tunnel for docker socket
	localPort := 2375
	remoteSocket := "/var/run/docker.sock"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := createSSHTunnel(ctx, client, localPort, remoteSocket)
		if err != nil {
			fmt.Printf("ssh tunnel error: %v\n", err)
		}
	}()

	// wait a moment for tunnel to establish
	time.Sleep(2 * time.Second)

	// cleanup any existing dozzle container
	fmt.Println("cleaning up existing dozzle container")
	_, _, _ = runLocalCommand("docker stop dozzle")
	_, _, _ = runLocalCommand("docker rm dozzle")

	// start dozzle container
	fmt.Println("starting dozzle container")
	dozzleCmd := fmt.Sprintf("docker run -d --name dozzle -p 8888:8080 -e DOCKER_HOST=tcp://host.docker.internal:%d amir20/dozzle:latest", localPort)
	_, _, err = runLocalCommand(dozzleCmd)
	if err != nil {
		return fmt.Errorf("failed to start dozzle container: %v", err)
	}

	// wait for dozzle to start
	time.Sleep(3 * time.Second)

	// open browser
	fmt.Println("opening dozzle ui in browser")
	err = openBrowser("http://localhost:8888")
	if err != nil {
		fmt.Printf("failed to open browser: %v\n", err)
		fmt.Println("dozzle ui available at: http://localhost:8888")
	}

	fmt.Println("dozzle ui is running")
	fmt.Println("press ctrl+c to stop")

	// keep the tunnel alive
	select {}
}

func createSSHTunnel(ctx context.Context, client *ssh.Client, localPort int, remoteSocket string) error {
	// listen on local port
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", localPort))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %v", localPort, err)
	}
	defer listener.Close()

	fmt.Printf("ssh tunnel listening on localhost:%d\n", localPort)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept connection: %v", err)
		}

		go func() {
			defer conn.Close()

			// connect to remote docker socket
			remoteConn, err := client.Dial("unix", remoteSocket)
			if err != nil {
				fmt.Printf("failed to connect to remote socket: %v\n", err)
				return
			}
			defer remoteConn.Close()

			// relay data bidirectionally
			go func() {
				defer conn.Close()
				defer remoteConn.Close()
				io.Copy(remoteConn, conn)
			}()

			defer conn.Close()
			defer remoteConn.Close()
			io.Copy(conn, remoteConn)
		}()
	}
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)

	return exec.Command(cmd, args...).Start()
}
