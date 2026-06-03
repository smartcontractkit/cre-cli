# Build hello-world workflow WASM, then serve the static demo with assets baked in.
FROM golang:1.26-bookworm AS wasm-builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go run ./web/wasm-simulate-demo/tools/build_assets

# Verify assets were produced (fail the image build if WASM is missing).
RUN test -f /src/web/wasm-simulate-demo/assets/hello-world.wasm \
    && test -f /src/web/wasm-simulate-demo/assets/manifest.json

FROM nginx:1.27-alpine AS runtime

COPY web/wasm-simulate-demo/nginx.conf /etc/nginx/conf.d/default.conf

# Static UI (vendor, JS, CSS) from context.
COPY web/wasm-simulate-demo/index.html \
     web/wasm-simulate-demo/wallet.html \
     web/wasm-simulate-demo/app.js \
     web/wasm-simulate-demo/wallet-app.js \
     web/wasm-simulate-demo/cre-host.js \
     web/wasm-simulate-demo/wasi-stubs.js \
     web/wasm-simulate-demo/payload.js \
     web/wasm-simulate-demo/styles.css \
     /usr/share/nginx/html/

COPY web/wasm-simulate-demo/lib/ /usr/share/nginx/html/lib/

COPY web/wasm-simulate-demo/assets/cre-environments.json \
     /usr/share/nginx/html/assets/

COPY web/wasm-simulate-demo/vendor/ /usr/share/nginx/html/vendor/

COPY web/wasm-simulate-demo/assets/headless-simulation.sample.json \
     /usr/share/nginx/html/assets/

# Prebuilt WASM + manifest from the Go builder stage.
COPY --from=wasm-builder /src/web/wasm-simulate-demo/assets/ /usr/share/nginx/html/assets/

EXPOSE 80

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1/assets/manifest.json || exit 1
