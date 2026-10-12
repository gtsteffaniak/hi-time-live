
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

# Starts a local coturn relay, then runs the ICE-failure specs against it so the
# relay-only test proves a relay candidate is really used. Needs host networking
# (Linux CI, or Docker Desktop with host networking enabled): without it the
# browser cannot reach the relay at 127.0.0.1. On a Mac where host networking is
# unavailable, run `make test-e2e` instead — it still runs the deterministic
# no-path test and skips the relay-only one. The relay is removed on success and
# on failure alike.
test-e2e-turn:
	cd tests/e2e && npm install && npx playwright install --with-deps chromium
	@docker rm -f hitime-coturn >/dev/null 2>&1 || true
	@trap 'docker rm -f hitime-coturn >/dev/null 2>&1 || true' EXIT; \
		docker run -d --name hitime-coturn --network host coturn/coturn:4.6.2 \
			-n --lt-cred-mech --user hitime:hitime --realm hitime \
			--no-cli --no-tls --no-dtls --fingerprint \
			--allow-loopback-peers --cli-password=hitime \
			--listening-port 3478 --min-port 49160 --max-port 49200 --log-file=stdout; \
		sleep 3; \
		cd tests/e2e && HITIME_TURN_URL=turn:127.0.0.1:3478 HITIME_TURN_USERNAME=hitime HITIME_TURN_CREDENTIAL=hitime \
			npx playwright test 08-ice-failure.spec.ts

lint:
	golangci-lint run

format:
	go fmt ./...