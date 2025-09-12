package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func runLocalCommand(fullCommand string) (string, string, error) {
	// Split the full command into parts
	parts := strings.Fields(fullCommand)
	if len(parts) == 0 {
		return "", "", fmt.Errorf("empty command string")
	}

	// First part is the command, the rest are the arguments
	command := parts[0]
	args := parts[1:]

	cmd := exec.Command(command, args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	fmt.Printf("> %s\n", fullCommand)

	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		fmt.Println(stderrBuf.String())
	}

	return stdoutBuf.String(), stderrBuf.String(), err
}

func BuildAndPushContainer(imageName string, tag string, platform string, buildArgFile string, target string) error {
	fmt.Println("building container")

	buildCmd := fmt.Sprintf("docker build --progress=plain --platform %s -t %s", platform, imageName)
	if buildArgFile != "" {
		args, err := parseBuildArgFile(buildArgFile)
		if err != nil {
			return err
		}

		for key, value := range args {
			buildCmd += fmt.Sprintf(" --build-arg %s=%s", key, value)
		}
	}
	if target != "" {
		buildCmd += fmt.Sprintf(" --target %s", target)
	}
	buildCmd += " ."

	_, _, err := runLocalCommand(buildCmd)
	if err != nil {
		return err
	}

	_, _, err = runLocalCommand(fmt.Sprintf("docker tag %s %s", imageName, tag))
	if err != nil {
		return err
	}

	fmt.Println("pushing container to registry")

	_, _, err = runLocalCommand(fmt.Sprintf("docker push %s", tag))
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

func parseBuildArgFile(filepath string) (map[string]string, error) {
	argMap := map[string]string{}

	contents, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(contents), "\n")

	for i, line := range lines {
		// skip empty or commented lines
		if (line == "") || strings.HasPrefix(line, "#") {
			continue
		}

		argLine := strings.Split(line, "=")

		if len(argLine) != 2 {
			return nil, fmt.Errorf("malformed build arg file at line %v", i)
		}

		argMap[argLine[0]] = argLine[1]
	}

	return argMap, nil
}
