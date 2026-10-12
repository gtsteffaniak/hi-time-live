
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

test-e2e:
	cd tests/e2e && npm install && npx playwright install --with-deps chromium && npx playwright test

test-e2e-base-path:
	cd tests/e2e && npm install && npx playwright install --with-deps chromium && HITIME_PORT=9013 HITIME_BASE_PATH=/hitime npx playwright test --grep-invert firefox

# Starts a local coturn relay, then runs the TURN-gated specs against it so the
# relay-only test proves a relay candidate is really used. Needs Linux host
# networking (CI) or Docker Desktop with host networking enabled; on a Mac the
# relay specs are skipped and the deterministic no-path test still runs.
test-e2e-turn:
	cd tests/e2e && npm install && npx playwright install --with-deps chromium
	docker rm -f hitime-coturn >/dev/null 2>&1 || true
	docker run -d --name hitime-coturn --network host coturn/coturn:4.6.2 \
		-n --lt-cred-mech --user hitime:hitime --realm hitime \
		--no-cli --no-tls --no-dtls --fingerprint \
		--listening-port 3478 --min-port 49160 --max-port 49200 --log-file=stdout
	sleep 3
	cd tests/e2e && HITIME_TURN_URL=turn:127.0.0.1:3478 HITIME_TURN_USERNAME=hitime HITIME_TURN_CREDENTIAL=hitime npx playwright test --grep "TURN"
	docker rm -f hitime-coturn >/dev/null 2>&1 || true

lint:
	golangci-lint run

format:
	go fmt ./...