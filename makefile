
build-docker:
	go mod vendor && cd .. && \
	go build --ldflags="-w -s -X github.com/gtsteffaniak/hi-time-live/routes.Version=v0.0.0-testing' .
	docker build -t hi-time-live .

run-docker:
	docker run -p 9012:9012 hi-time-live

# first run : export GOROOT=$(go env GOROOT)
setup:
	export GOROOT=$(go env GOROOT)
	go run $(GOROOT)/src/crypto/tls/generate_cert.go --host localhost

run:
	go run --ldflags="-w -s -X github.com/gtsteffaniak/hi-time-live/routes.Version=v0.0.0-testing" . --dev

test:
	go test -v --race ./...

lint:
	golangci-lint run

format:
	go fmt ./...