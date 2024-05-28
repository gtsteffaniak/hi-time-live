
build-docker:
	cd site && go mod vendor && cd .. && \
	docker build -t hi-time-live .

run-docker:
	docker run -p 9012:9012 hi-time-live

# first run : export GOROOT=$(go env GOROOT)
setup:
	export GOROOT=$(go env GOROOT)
	go run $(GOROOT)/src/crypto/tls/generate_cert.go --host localhost && \
	mv *.pem ./site/

run:
	cd site && go run . --dev

test:
	cd site && go test -v --race ./...

lint:
	cd site && golangci-lint run

format:
	cd site && go fmt ./...