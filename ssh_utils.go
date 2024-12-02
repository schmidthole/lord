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

func withSSHClient(address string, f func(*ssh.Client) error) error {
	fmt.Printf("connecting server: %s\n", address)

	client, err := getSSHClient(address)
	if err != nil {
		return err
	}
	defer client.Close()

	return f(client)
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

func sftpCopyFileFromRemote(client *ssh.Client, srcFilePath string, dstFilePath string) error {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	srcFile, err := sftpClient.Open(srcFilePath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstFilePath)
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
