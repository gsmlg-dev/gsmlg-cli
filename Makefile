.PHONY: setup-dev setup-ci


setup-dev:
	@go mod edit -replace github.com/gsmlg-dev/gsmlg-golang=../gsmlg-golang

setup-ci:
	@git clone https://github.com/gsmlg-dev/gsmlg-golang.git gsmlg-golang
	@go mod edit -replace github.com/gsmlg-dev/gsmlg-golang=./gsmlg-golang
