SOURCES := $(wildcard *.go) go.mod go.sum

lord: $(SOURCES)
	go build -o lord .

build: lord

install: lord
	sudo cp lord /usr/local/bin

clean:
	rm -f lord

format:
	gofmt -w *.go

test:
	go test -v ./...
