package main

import (
	"flag"
	"fmt"
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

func main() {
	fmt.Println(banner)

	deployFlag := flag.Bool("deploy", false, "build and deploy the container")
	logsFlag := flag.Bool("logs", false, "get logs from the running container")
	initFlag := flag.Bool("init", false, "initialize lord config in current directory")
	destroyFLag := flag.Bool("destroy", false, "stop and delete a running container")
	statusFlag := flag.Bool("status", false, "get the status of a running container")

	flag.Parse()

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

	if *deployFlag {
		imageTag := fmt.Sprintf("%s/%s:latest", c.Registry, c.Name)

		err = BuildAndPushContainer(c.Name, imageTag, c.Platform)
		if err != nil {
			panic(err)
		}

		fmt.Println("checking server state")

		err = server.ensureDockerInstalled(c.Username, c.Password)
		if err != nil {
			panic(err)
		}

		err = server.ensureDockerRunning()
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

		err = server.runContainer(c.Name, imageTag, c.Volumes)
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
	} else {
		fmt.Println("No command specified\n\nUsage:")
		flag.VisitAll(func(f *flag.Flag) {
			fmt.Printf("-%s: %s\n", f.Name, f.Usage)
		})
	}
}
