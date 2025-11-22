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

var version = "v1.5.0"

func main() {
	fmt.Println(banner)

	configFlag := flag.String("config", "", "specify a lord config key to use (i.e. set to \"beta\" to pickup the beta.lord.yml file)")
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
	logDownloadFlag := flag.Bool("logdownload", false, "download log file from the server")
	monitorFlag := flag.Bool("monitor", false, "get system stats from the server")
	registryFlag := flag.Bool("registry", false, "ensure the container registry can be authenticated on the host, including installing platform specific login tools")
	dozzleFlag := flag.Bool("dozzle", false, "open dozzle web ui for monitoring containers via ssh tunnel")
	diffFlag := flag.Bool("diff", false, "compare local files with deployed files on the server")

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

	c, err := loadConfig(*configFlag)
	if err != nil {
		printConsoleError("error loading lord config", err)
	}

	server := remote{c.Server, c}

	if *serverFlag || *deployFlag || *recoverFlag {
		fmt.Println("checking server state")

		err = server.ensureLordSetup()
		if err != nil {
			printConsoleError("error with initial setup of the remote server", err)
		}

		err = server.ensureDockerInstalled(*recoverFlag)
		if err != nil {
			printConsoleError("error installing docker on the remote server", err)
		}

		err = server.ensureDockerRunning()
		if err != nil {
			printConsoleError("error running docker on the remote server", err)
		}
	}

	if (*serverFlag || *deployFlag || *recoverFlag || *registryFlag) && c.Registry != "" {
		err = server.ensureRegistryAuthenticated(*recoverFlag)
		if err != nil {
			printConsoleError("error authenticating to registry", err)
		}
	}

	if *serverFlag || *deployFlag || *recoverFlag || *proxyFlag {
		// only check traefik if we are deploying a web container
		if c.Web {
			err = server.ensureTraefikSetup(c.Email)
			if err != nil {
				printConsoleError("error setting up reverse proxy on remote server", err)
			}
		}
	}

	if *deployFlag {
		var imageTag string
		if c.Registry == "" {
			imageTag = fmt.Sprintf("lorddirect/%s:latest", c.Name)
		} else {
			imageTag = fmt.Sprintf("%s/%s:latest", c.Registry, c.Name)
		}

		if c.Registry == "" {
			err = BuildAndSaveContainer(c.Name, imageTag, c.Platform, c.BuildArgFile, c.Target)

			if err != nil {
				printConsoleError("error building and saving the container", err)
			}
		} else {
			err = BuildAndPushContainer(c.Name, imageTag, c.Platform, c.BuildArgFile, c.Target)

			if err != nil {
				printConsoleError("error building and pusing container to registry", err)
			}
		}

		fmt.Println("updating and running container on server")

		err = server.stageForContainer(c.Name, c.Volumes, c.EnvironmentFile)
		if err != nil {
			printConsoleError("error staging remote server for running the container", err)
		}

		if c.Registry == "" {
			fmt.Println("direct loading container to server. this could take awhile...")
			err = server.directLoadContainer(c.Name)

			if err != nil {
				printConsoleError("error direct loading container onto remote server", err)
			}
		} else {
			fmt.Println("pulling container from registry onto server")
			err = server.pullContainer(imageTag)

			if err != nil {
				printConsoleError("error pulling container from registry on remote server", err)
			}
		}

		if c.Registry == "" {
			err = DeleteSavedContainer(c.Name)
			if err != nil {
				fmt.Printf("warning: failed to cleanup local container file: %v\n", err)
			}
		}

		err = server.stopAndDeleteContainer(c.Name)
		if err != nil {
			printConsoleError("error stopping/deleting container on remote server", err)
		}

		err = server.runContainer(c.Name, imageTag, c.Volumes, c.EnvironmentFile, c.Web, c.Hostname)
		if err != nil {
			printConsoleError("error runing container on remote server", err)
		}

		fmt.Println("finished deployment")
	} else if *logsFlag {
		err = server.streamContainerLogs(c.Name)
		if err != nil {
			printConsoleError("error streaming container logs", err)
		}
	} else if *destroyFLag {
		err = server.stopAndDeleteContainer(c.Name)
		if err != nil {
			printConsoleError("error stopping/deleting container on remote server", err)
		}
	} else if *statusFlag {
		err = server.getContainerStatus(c.Name)
		if err != nil {
			printConsoleError("error getting container status on remote server", err)
		}
	} else if *logDownloadFlag {
		initLocalLogDirectory()
		if err != nil {
			printConsoleError("error creating local log storage directory", err)
		}

		err = server.downloadContainerLogs(c.Name)
		if err != nil {
			printConsoleError("error downloading logs from remote server", err)
		}
	} else if *monitorFlag {
		err = server.getSystemStats(false)
		if err != nil {
			printConsoleError("error getting system stats from remote server", err)
		}
	} else if *dozzleFlag {
		err = startDozzleUI(c.Server, c)
		if err != nil {
			printConsoleError("error starting and connecting dozzle ui", err)
		}
	} else if *diffFlag {
		err = server.diffLocalAndRemote(c.Name)
		if err != nil {
			printConsoleError("error comparing local and remote files", err)
		}
	} else if !*serverFlag && !*recoverFlag && !*proxyFlag {
		fmt.Println("not a valid command")
	}
}
