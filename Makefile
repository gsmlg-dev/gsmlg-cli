.PHONY: setup-dev setup-ci build
.EXPORT_ALL_VARIABLES:

CGO_ENABLE := 0
GOOS := ${GOOS}
GOARCH := ${GOARCH}

setup-dev:
	@go mod edit -replace github.com/gsmlg-dev/gsmlg-golang=../gsmlg-golang

setup-ci:
	@git clone https://github.com/gsmlg-dev/gsmlg-golang.git gsmlg-golang
	@go mod edit -replace github.com/gsmlg-dev/gsmlg-golang=./gsmlg-golang

build:
	@echo GO Build: $$GOOS $$GOARCH $$CGO_ENABLE
	@go build -trimpath -o gsmlg-cli main.go

clean:
	@rm -rf gsmlg-golang
	@rm -rf gsmlg-cli
