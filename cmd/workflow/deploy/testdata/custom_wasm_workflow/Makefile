.PHONY: build

export GOOS := wasip1
export GOARCH := wasm
export CGO_ENABLED := 0

build:
	go build -o wasm/workflow.wasm -trimpath -ldflags="-buildid= -w -s" .
