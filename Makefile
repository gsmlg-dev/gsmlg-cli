.PHONY: setup-dev setup-ci build
.EXPORT_ALL_VARIABLES:

CGO_ENABLED := 0
GOOS := ${GOOS}
GOARCH := ${GOARCH}
VERSION := dev

setup-dev:
	@go mod edit -replace github.com/gsmlg-dev/gsmlg-golang=../gsmlg-golang

setup-ci:
	@git clone https://github.com/gsmlg-dev/gsmlg-golang.git gsmlg-golang
	@go mod edit -replace github.com/gsmlg-dev/gsmlg-golang=./gsmlg-golang

build:
	@echo GO Build: $$GOOS $$GOARCH CGO_ENABLED=$$CGO_ENABLED
	@go build -trimpath -ldflags "-w -s -X github.com/gsmlg-dev/gsmlg-cli/cmd.Version=$(VERSION)" -o gsmlg-cli main.go

clean:
	@rm -rf gsmlg-golang
	@rm -rf gsmlg-cli
