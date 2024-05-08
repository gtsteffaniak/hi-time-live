
build:
	cd site && go mod vendor && cd .. && \
	docker build -t hi-time-live .

run:
	cd site && go run .

test:
	cd site && go test -v --race ./...