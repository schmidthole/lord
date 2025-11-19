package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func getSSHClient(server string, config *Config) (*ssh.Client, error) {
	authMethod, err := getAuthMethod(config)
	if err != nil {
		panic(err)
	}

	user := "root"
	if config.User != "" {
		user = config.User
	}

	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	return ssh.Dial("tcp", fmt.Sprintf("%s:22", server), sshConfig)
}

func getAuthMethod(config *Config) (ssh.AuthMethod, error) {
	var sshKeyPath string

	if config.SshKeyFile != "" {
		sshKeyPath = config.SshKeyFile
	} else {
		defaultKeyPaths := []string{
			filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519"),
			filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa"),
			filepath.Join(os.Getenv("HOME"), ".ssh", "id_dsa"),
		}

		for _, keyPath := range defaultKeyPaths {
			_, err := os.Stat(keyPath)
			if err == nil {
				sshKeyPath = keyPath
				break
			}
		}
	}

	if sshKeyPath == "" {
		return nil, fmt.Errorf("no ssh key found")
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

func withSSHClient(address string, config *Config, f func(*ssh.Client) error) error {
	fmt.Printf("connecting server: %s\n", address)

	client, err := getSSHClient(address, config)
	if err != nil {
		return err
	}
	defer client.Close()

	return f(client)
}

func runSSHCommand(client *ssh.Client, cmd string, appName string) (string, string, error) {
	return runSSHCommandWithOutput(client, cmd, appName, true)
}

func runSSHCommandSilent(client *ssh.Client, cmd string, appName string) (string, string, error) {
	return runSSHCommandWithOutput(client, cmd, appName, false)
}

func runSSHCommandWithOutput(client *ssh.Client, cmd string, appName string, verbose bool) (string, string, error) {
	session, err := client.NewSession()
	if err != nil {
		panic(err)
	}
	defer session.Close()

	// source app-specific environment variables if they exist
	var fullCmd string
	if appName != "" {
		fullCmd = fmt.Sprintf("test -f /etc/lord/%s && source /etc/lord/%s; %s", appName, appName, cmd)
	} else {
		fullCmd = cmd
	}

	if verbose {
		fmt.Printf("> %s\n", cmd)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	err = session.Run(fullCmd)
	if err != nil {
		if verbose {
			fmt.Println(stderrBuf.String())
		}
		return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("command execution failed: %v", err)
	}

	if verbose {
		fmt.Println(stdoutBuf.String())
	}

	return stdoutBuf.String(), stderrBuf.String(), nil
}

func sftpCopyFileToRemote(client *ssh.Client, srcFilePath string, dstFilePath string) error {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	srcFile, err := os.Open(srcFilePath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := sftpClient.Create(dstFilePath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	return nil
}
