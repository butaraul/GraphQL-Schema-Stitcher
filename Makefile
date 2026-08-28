.PHONY: run run-gateway run-mocks generate test build tidy clean

GOFLAGS :=

## run: start all 3 mock services + the gateway together, wired to talk to each other.
run:
	@trap 'kill 0' EXIT INT TERM; \
	go run ./cmd/mockuser & \
	go run ./cmd/mockpost & \
	go run ./cmd/mockcomment & \
	sleep 1; \
	go run ./cmd/gateway & \
	wait

## run-mocks: start only the 3 mock downstream services.
run-mocks:
	@trap 'kill 0' EXIT INT TERM; \
	go run ./cmd/mockuser & \
	go run ./cmd/mockpost & \
	go run ./cmd/mockcomment & \
	wait

## run-gateway: start only the gateway (expects the 3 services to already be running).
run-gateway:
	go run ./cmd/gateway

## generate: regenerate gqlgen's exec/model/resolver-stub code from internal/schema/*.graphqls.
generate:
	go run github.com/99designs/gqlgen generate

## test: run the full test suite with the race detector.
test:
	go test ./... -race -count=1

## build: compile all binaries into ./bin.
build:
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/mockuser ./cmd/mockuser
	go build -o bin/mockpost ./cmd/mockpost
	go build -o bin/mockcomment ./cmd/mockcomment

## tidy: sync go.mod/go.sum.
tidy:
	go mod tidy

## clean: remove build artifacts.
clean:
	rm -rf bin
