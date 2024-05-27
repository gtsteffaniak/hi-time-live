
build:
	cd site && go mod vendor && cd - && \
	docker build -t hi-time-live .

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