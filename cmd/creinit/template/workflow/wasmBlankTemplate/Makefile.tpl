.PHONY: build

build:
	# TODO: Add your build logic here
	# This target should compile your workflow to wasm/workflow.wasm
	# Example for Go:
	# GOOS=wasip1 GOARCH=wasm go build -o wasm/workflow.wasm .
	# Example for Rust:
	# cargo build --target wasm32-wasi --release
	# cp target/wasm32-wasi/release/workflow.wasm wasm/workflow.wasm
	@echo "Please implement the build target in the Makefile"
	@exit 1
