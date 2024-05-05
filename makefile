
build:
	cd backend && go mod vendor && cd .. && \
	docker build -t hi-time-live .

run:
	cd backend && go run .

test:
	cd backend && go test -v --race ./...