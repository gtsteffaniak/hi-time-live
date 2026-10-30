package routes

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
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
	const marker = `id="ice-config"`
	abs := strings.Index(body, marker)
	if abs < 0 {
		t.Fatalf("room page did not render an ICE server list")
	}
	afterTag := strings.Index(body[abs:], ">")
	if afterTag < 0 {
		t.Fatalf("ICE server list element is malformed")
	}
	start := abs + afterTag + 1
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
	if err := ConfigureICE(false, "", "", "", "", "secret", time.Hour, ""); err == nil {
		t.Fatal("expected error when TURN_REST_SECRET is set without TURN_URL outside dev mode")
	}
}

func TestInvalidTurnURLIsRejected(t *testing.T) {
	resetICEConfig(t)
	if err := ConfigureICE(true, "", "http://turn.example.com:3478", "u", "p", "", 0, ""); err == nil {
		t.Fatal("expected invalid TURN_URL to be rejected")
	}
}

func TestDevTurnURLRequiresStaticCredentials(t *testing.T) {
	resetICEConfig(t)
	if err := ConfigureICE(true, "", "turn:turn.example.com:3478", "alice", "", "", 0, ""); err == nil {
		t.Fatal("expected error when only TURN_USERNAME is set")
	}
}

func TestICEServersRejectsNonICESchemes(t *testing.T) {
	resetICEConfig(t)
	if err := ConfigureICE(true, `[{"urls":"https://example.com"}]`, "", "", "", "", 0, ""); err == nil {
		t.Fatal("expected non-ICE URL scheme to be rejected")
	}
}

func TestIceConfigEndpointRefreshesRESTCredentials(t *testing.T) {
	resetICEConfig(t)
	server := newTestServer(t)
	if err := ConfigureICE(false, "", "turn:turn.example.com:3478", "", "", "shared-secret", time.Hour, "hitime"); err != nil {
		t.Fatalf("ConfigureICE: %v", err)
	}
	code := newCode(t)
	resp, err := server.Client().Get(server.URL + "/ice-config?code=" + code)
	if err != nil {
		t.Fatalf("GET ice-config: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	var servers []ICEServer
	if err := json.Unmarshal(body, &servers); err != nil {
		t.Fatalf("decoding ice-config: %v", err)
	}
	var turn *ICEServer
	for i := range servers {
		for _, u := range iceServerURLs(servers[i].URLs) {
			if strings.Contains(u, "turn:turn.example.com:3478") {
				turn = &servers[i]
				break
			}
		}
	}
	if turn == nil {
		t.Fatalf("ice-config missing TURN entry: %s", body)
	}
	if turn.Username == "" || turn.Credential == "" {
		t.Fatalf("ice-config TURN entry lacks REST credential: %s", body)
	}
	if strings.Contains(string(body), "shared-secret") {
		t.Fatalf("ice-config exposed the shared secret: %s", body)
	}
}

func TestTurnURLRejectsStunScheme(t *testing.T) {
	resetICEConfig(t)
	if err := ConfigureICE(true, "", "stun:stun.example.com:3478", "u", "p", "", 0, ""); err == nil {
		t.Fatal("expected STUN URL in TURN_URL to be rejected")
	}
}

func TestProductionIgnoresUnusedTurnURLValidation(t *testing.T) {
	resetICEConfig(t)
	server := newTestServer(t)
	if err := ConfigureICE(false, "", "http://not-turn.example.com:3478", "alice", "s3cret", "", 0, ""); err != nil {
		t.Fatalf("production should ignore unused TURN_URL even when malformed: %v", err)
	}
	line := renderedIceServers(t, roomPage(t, server, server.URL+"/room?id="+newCode(t)))
	if strings.Contains(line, "not-turn.example.com") {
		t.Fatalf("ignored TURN_URL must not appear on the page: %s", line)
	}
}

func TestInvalidTurnURLMissingHostIsRejected(t *testing.T) {
	resetICEConfig(t)
	if err := ConfigureICE(true, "", "turn:", "u", "p", "", 0, ""); err == nil {
		t.Fatal("expected bare turn: URL to be rejected")
	}
}

func TestICEServersRejectsBlankMemberInArray(t *testing.T) {
	resetICEConfig(t)
	if err := ConfigureICE(true, `[{"urls":["stun:custom.example.com:3478",""]}]`, "", "", "", "", 0, ""); err == nil {
		t.Fatal("expected blank ICE_SERVERS url member to be rejected")
	}
}
