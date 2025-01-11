
build-docker:
	go mod vendor && cd .. && \
	docker build -t hi-time-live .

run-docker:
	docker run -p 9012:9012 hi-time-live

# first run : export GOROOT=$(go env GOROOT)
setup:
	export GOROOT=$(go env GOROOT)
	go run $(GOROOT)/src/crypto/tls/generate_cert.go --host localhost

run:
	go run . --dev

test:
	go test -v --race ./...

lint:
	golangci-lint run

format:
	go fmt ./...