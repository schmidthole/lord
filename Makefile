lord: main.go
	go build -o lord .

build: lord

install: lord
	sudo cp lord /usr/local/bin

clean:
	rm -f lord
