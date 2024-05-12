
build:
	cd site && go mod vendor && cd .. && \
	docker build -t hi-time-live .

# first run : export GOROOT=$(go env GOROOT)
setup:
	go run $(GOROOT)/src/crypto/tls/generate_cert.go --host localhost & \
	mv *.pem ./site/

run:
	cd site && go run .

test:
	cd site && go test -v --race ./...