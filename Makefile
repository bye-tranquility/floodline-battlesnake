.PHONY: run build cli test bench

run:
	go run .

build:
	mkdir -p .work/bin
	go build -o .work/bin/floodline .

cli:
	mkdir -p .work/bin
	GOBIN="$(CURDIR)/.work/bin" go install github.com/BattlesnakeOfficial/rules/cli/battlesnake@v1.2.3

test:
	go test ./...

bench:
	python3 tools/loadtest.py
