package main

import (
	"flag"
	"fmt"
)

var banner = `
 __         ______     ______     _____    
/\ \       /\  __ \   /\  == \   /\  __-.  
\ \ \____  \ \ \/\ \  \ \  __<   \ \ \/\ \ 
 \ \_____\  \ \_____\  \ \_\ \_\  \ \____- 
  \/_____/   \/_____/   \/_/ /_/   \/____/ 
                                           
`

var version = "1"

func main() {
	fmt.Println(banner)

	deployFlag := flag.Bool("deploy", false, "build and deploy the container")
	logsFlag := flag.Bool("logs", false, "get logs from the running container")
	initFlag := flag.Bool("init", false, "initialize lord config in current directory")
	serverFlag := flag.Bool("server", false, "only runs/checks the server setup, will setup the proxy as well")
	destroyFLag := flag.Bool("destroy", false, "stop and delete a running container")
	statusFlag := flag.Bool("status", false, "get the status of a running container")
	helpFlag := flag.Bool("help", false, "get help with commands")
	versionFlag := flag.Bool("version", false, "get lord version")
	recoverFlag := flag.Bool("recover", false, "attempt to recover a server that has a bad install")
	proxyFlag := flag.Bool("proxy", false, "only runs/checks the proxy setup")

	flag.Parse()

	noFlagsSet := true
	flag.VisitAll(func(f *flag.Flag) {
		if f.Value.String() == "true" {
			noFlagsSet = false
		}
	})

	if *helpFlag || noFlagsSet {
		if noFlagsSet {
			fmt.Println("No command specified\n\nUsage:")
		}

		displayVerison()
		displayHelp()
		return
	}

	if *versionFlag {
		displayVerison()
		return
	}

	if *initFlag {
		err := initLocalProject()
		if err != nil {
			panic(err)
		}

		return
	}

	c, err := loadConfig()
	if err != nil {
		panic(err)
	}

	server := remote{c.Server}

	if *serverFlag || *deployFlag || *recoverFlag {
		fmt.Println("checking server state")

		err = server.ensureDockerInstalled(c.AuthFile, *recoverFlag)
		if err != nil {
			panic(err)
		}

		err = server.ensureDockerRunning()
		if err != nil {
			panic(err)
		}

	}

	if *serverFlag || *deployFlag || *recoverFlag || *proxyFlag {
		err = server.ensureTraefikSetup(c.Email)
		if err != nil {
			panic(err)
		}
	}

	if *deployFlag {
		imageTag := fmt.Sprintf("%s/%s:latest", c.Registry, c.Name)

		err = BuildAndPushContainer(c.Name, imageTag, c.Platform)
		if err != nil {
			panic(err)
		}

		fmt.Println("updating and running container on server")

		err = server.pullContainer(imageTag)
		if err != nil {
			panic(err)
		}

		err = server.stopAndDeleteContainer(c.Name)
		if err != nil {
			panic(err)
		}

		err = server.runContainer(c.Name, imageTag, c.Volumes, c.Web, c.Hostname)
		if err != nil {
			panic(err)
		}

		fmt.Println("finished deployment")
	} else if *logsFlag {
		err = server.streamContainerLogs(c.Name)
		if err != nil {
			panic(err)
		}
	} else if *destroyFLag {
		err = server.stopAndDeleteContainer(c.Name)
		if err != nil {
			panic(err)
		}
	} else if *statusFlag {
		err = server.getContainerStatus(c.Name)
		if err != nil {
			panic(err)
		}
	} else if !*serverFlag && !*recoverFlag && !*proxyFlag {
		panic(fmt.Errorf("not a valid command"))
	}
}
