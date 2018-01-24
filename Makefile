.PHONY: fmt

fmt:
	go fmt ./...

# https://golang.org/doc/install/source#environment
release:
	GOOS=linux go build -o tun2socks_linux cmd/main.go
	GOOS=linux GOARCH=arm go build -o tun2socks_linux_arm cmd/main.go
	GOOS=darwin go build -o tun2socks_darwin cmd/main.go
	GOOS=windows GOARCH=amd64 go build -o tun2socks_windows_64.exe cmd/main.go
	GOOS=windows GOARCH=386 go build -o tun2socks_windows_32.exe cmd/main.go

shared:
	go build -buildmode=c-shared -o libtun2socks.so ./cmd

static:
	go build -buildmode=c-archive -o libtun2socks.a ./cmd
