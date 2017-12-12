.PHONY: fmt

fmt:
	go fmt ./...

release:
	GOOS=linux go build -o tun2socks_linux cmd/main.go
	GOOS=linux GOARCH=arm go build -o tun2socks_linux_arm cmd/main.go
	GOOS=darwin go build -o tun2socks_darwin cmd/main.go
	GOOS=windows go build -o tun2socks_windows.exe cmd/main.go
