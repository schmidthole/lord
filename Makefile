lord: config.go local.go main.go remote.go ssh_utils.go traefik.go go.mod go.sum  registry.go system_monitor.go
	go build -o lord .

build: lord

install: lord
	sudo cp lord /usr/local/bin

clean:
	rm -f lord
