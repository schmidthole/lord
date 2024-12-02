package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func runLocalCommand(command string, args ...string) (string, string, error) {
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

func BuildAndPushContainer(imageName string, tag string, platform string) error {
	fmt.Println("building container")

	_, _, err := runLocalCommand("docker", "build", "--progress=plain", "--platform", platform, "-t", imageName, ".")
	if err != nil {
		return err
	}

	_, _, err = runLocalCommand("docker", "tag", imageName, tag)
	if err != nil {
		return err
	}

	fmt.Println("pushing container to registry")

	_, _, err = runLocalCommand("docker", "push", tag)
	if err != nil {
		return err
	}

	return nil
}

func initLocalProject() error {
	_, err := os.Stat("lord.yml")
	if err == nil {
		fmt.Println("lord already initialized in current directory")
		return nil
	} else if os.IsNotExist(err) {
		err := os.WriteFile("lord.yml", []byte(baseConfig), 0644)
		if err != nil {
			return err
		} else {
			fmt.Println("lord initialized successfully in current directory")
			return nil
		}
	} else {
		return fmt.Errorf("error initializing lord config: %v", err)
	}
}

func displayHelp() {
	flag.VisitAll(func(f *flag.Flag) {
		fmt.Printf("-%s: %s\n", f.Name, f.Usage)
	})
}

func displayVerison() {
	fmt.Printf("\n version: %s\n\n\n", version)
}

func initLocalLogDirectory() error {
	_, err := os.Stat("lord-logs")
	if os.IsNotExist(err) {
		fmt.Println("creating lord-logs directory")
		err = os.Mkdir("./lord-logs", 0755)
	}

	return err
}
