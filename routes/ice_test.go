package routes

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func resetICEConfig(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		activeICE = iceConfig{}
	})
	activeICE = iceConfig{}
}

func roomPage(t *testing.T, server *httptest.Server, url string) string {
	t.Helper()
	resp, err := server.Client().Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return string(body)
}

func renderedIceServers(t *testing.T, body string) string {
	t.Helper()
	const marker = `<script type="application/json" id="ice-config">`
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("room page did not render an ICE server list")
	}
	start += len(marker)
	end := strings.Index(body[start:], "</script>")
	if end < 0 {
		t.Fatalf("ICE server list element is not closed")
	}
	return body[start : start+end]
}

func TestRoomPageRendersConfiguredTurnServer(t *testing.T) {
	resetICEConfig(t)
	server := newTestServer(t)
	if err := ConfigureICE(true, "", "turn:turn.example.com:3478", "alice", "s3cret", "", 0, ""); err != nil {
		t.Fatalf("ConfigureICE: %v", err)
	}
	line := renderedIceServers(t, roomPage(t, server, server.URL+"/room?id="+newCode(t)))

	for _, want := range []string{"turn:turn.example.com:3478", `"username":"alice"`, `"credential":"s3cret"`} {
		if !strings.Contains(line, want) {
			t.Errorf("rendered ICE servers missing %q: %s", want, line)
		}
	}
	if !strings.Contains(line, "stun:stun.l.google.com:19302") {
		t.Errorf("rendered ICE servers dropped the default STUN servers: %s", line)
	}
}

func TestICEServersOverrideReplacesDefaults(t *testing.T) {
	resetICEConfig(t)
	server := newTestServer(t)
	override := `[{"urls":["stun:custom.example.com:3478","turns:turn.example.com:5349"],"username":"u","credential":"p"}]`
	if err := ConfigureICE(true, override, "turn:ignored.example.com:3478", "ignored", "ignored", "", 0, ""); err != nil {
		t.Fatalf("ConfigureICE: %v", err)
	}
	line := renderedIceServers(t, roomPage(t, server, server.URL+"/room?id="+newCode(t)))

	if !strings.Contains(line, "stun:custom.example.com:3478") || !strings.Contains(line, "turns:turn.example.com:5349") {
		t.Errorf("ICE_SERVERS override not rendered: %s", line)
	}
	if strings.Contains(line, "stun:stun.l.google.com:19302") {
		t.Errorf("override should replace the defaults, but they are still present: %s", line)
	}
}

func TestInvalidICEServersIsRejected(t *testing.T) {
	resetICEConfig(t)
	cases := map[string]string{
		"malformed JSON":     "{not json",
		"missing urls":       `[{}]`,
		"empty url list":     `[{"urls":[]}]`,
		"blank url string":   `[{"urls":""}]`,
		"blank url in array": `[{"urls":["  "]}]`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if err := ConfigureICE(true, raw, "", "", "", "", 0, ""); err == nil {
				t.Fatalf("ConfigureICE accepted invalid ICE_SERVERS: %s", raw)
			}
		})
	}
}

func TestTurnRESTCredentialMatchesCoturn(t *testing.T) {
	secret := "test-secret"
	ttl := time.Hour
	user, pass := turnRESTCredential(secret, ttl, "room-user")
	if !strings.Contains(user, ":") {
		t.Fatalf("username %q should contain expiry and suffix", user)
	}
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(user))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if pass != want {
		t.Fatalf("credential = %q, want %q", pass, want)
	}
}

func TestProductionTurnRESTWithoutStaticSecret(t *testing.T) {
	resetICEConfig(t)
	server := newTestServer(t)
	if err := ConfigureICE(false, "", "turn:turn.example.com:3478", "", "", "shared-secret", time.Hour, "hitime"); err != nil {
		t.Fatalf("ConfigureICE: %v", err)
	}
	line := renderedIceServers(t, roomPage(t, server, server.URL+"/room?id="+newCode(t)))
	if !strings.Contains(line, "turn:turn.example.com:3478") {
		t.Fatalf("TURN URL not rendered in production REST mode: %s", line)
	}
	if !strings.Contains(line, `"credential":`) || strings.Contains(line, "shared-secret") {
		t.Fatalf("page must expose REST credential but not the shared secret: %s", line)
	}
}

func TestProductionIgnoresStaticTurnCredential(t *testing.T) {
	resetICEConfig(t)
	server := newTestServer(t)
	if err := ConfigureICE(false, "", "turn:turn.example.com:3478", "alice", "s3cret", "", 0, ""); err != nil {
		t.Fatalf("ConfigureICE: %v", err)
	}
	line := renderedIceServers(t, roomPage(t, server, server.URL+"/room?id="+newCode(t)))
	if strings.Contains(line, "turn.example.com") {
		t.Fatalf("static TURN credentials must not be published outside dev mode: %s", line)
	}
	if !strings.Contains(line, "stun:stun.l.google.com:19302") {
		t.Fatalf("expected default STUN servers: %s", line)
	}
}

func TestTurnRESTRequiresTurnURL(t *testing.T) {
	resetICEConfig(t)
	if err := ConfigureICE(true, "", "", "", "", "secret", time.Hour, ""); err == nil {
		t.Fatal("expected error when TURN_REST_SECRET is set without TURN_URL")
	}
}
