lord: config.go local.go main.go remote.go ssh_utils.go go.mod go.sum
	go build -o lord .

build: lord

install: lord
	sudo cp lord /usr/local/bin

clean:
	rm -f lord
