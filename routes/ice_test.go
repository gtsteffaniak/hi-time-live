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
	"strconv"
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
	fixed := time.Unix(1_700_000_000, 0)
	nowFunc = func() time.Time { return fixed }
	t.Cleanup(func() { nowFunc = time.Now })

	first := fetchIceConfigTurn(t, server, code)
	secondTime := fixed.Add(30 * time.Minute)
	nowFunc = func() time.Time { return secondTime }
	second := fetchIceConfigTurn(t, server, code)

	exp1, err := turnRESTUsernameExpiry(first.Username)
	if err != nil {
		t.Fatalf("first username: %v", err)
	}
	exp2, err := turnRESTUsernameExpiry(second.Username)
	if err != nil {
		t.Fatalf("second username: %v", err)
	}
	if exp2 <= exp1 {
		t.Fatalf("second expiry %d should be after first %d", exp2, exp1)
	}
	if second.Credential == first.Credential {
		t.Fatal("expected refreshed REST credential to change")
	}
}

func fetchIceConfigTurn(t *testing.T, server *httptest.Server, code string) *ICEServer {
	t.Helper()
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
	return turn
}

func turnRESTUsernameExpiry(username string) (int64, error) {
	part := strings.SplitN(username, ":", 2)[0]
	return strconv.ParseInt(part, 10, 64)
}

func TestIceConfigEndpointRejectsInvalidCode(t *testing.T) {
	resetICEConfig(t)
	server := newTestServer(t)
	if err := ConfigureICE(false, "", "turn:turn.example.com:3478", "", "", "shared-secret", time.Hour, "hitime"); err != nil {
		t.Fatalf("ConfigureICE: %v", err)
	}
	resp, err := server.Client().Get(server.URL + "/ice-config?code=invalid!")
	if err != nil {
		t.Fatalf("GET ice-config: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestTurnRESTSecretNotTrimmedForHMAC(t *testing.T) {
	secret := " padded-secret "
	user, pass := turnRESTCredential(secret, time.Hour, "room")
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(user))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if pass != want {
		t.Fatalf("credential must use untrimmed secret bytes")
	}
}

func TestInvalidIceURLEmptyHostRejected(t *testing.T) {
	resetICEConfig(t)
	if err := ConfigureICE(true, "", "turn::3478", "u", "p", "", 0, ""); err == nil {
		t.Fatal("expected turn URL with empty host to be rejected")
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

func TestICEServersAcceptsSingleURLString(t *testing.T) {
	resetICEConfig(t)
	if err := ConfigureICE(true, `[{"urls":"stun:only.example.com:3478"}]`, "", "", "", "", 0, ""); err != nil {
		t.Fatalf("ConfigureICE rejected a single-string urls value: %v", err)
	}
	if page := string(iceServersForPage()); !strings.Contains(page, "stun:only.example.com:3478") {
		t.Errorf("single-string urls value was not rendered: %s", page)
	}
}

// The overrides are a development-only feature: a production run must never
// publish a relay credential on a room page, because anyone who can load any
// room URL can read it and reuse the relay outside this app.
func TestICEOverridesAreIgnoredOutsideDevMode(t *testing.T) {
	resetICEConfig(t)
	server := newTestServer(t)
	err := ConfigureICE(false,
		`[{"urls":"turn:relay.example.com:3478","username":"alice","credential":"s3cret"}]`,
		"turn:relay.example.com:3478", "alice", "s3cret", "", 0, "")
	if err != nil {
		t.Fatalf("ConfigureICE outside dev mode returned an error: %v", err)
	}

	line := renderedIceServers(t, roomPage(t, server, server.URL+"/room?id="+newCode(t)))
	for _, leaked := range []string{"s3cret", "relay.example.com"} {
		if strings.Contains(line, leaked) {
			t.Errorf("a production run published the configured relay (%q): %s", leaked, line)
		}
	}
	if !strings.Contains(line, "stun:stun.l.google.com:19302") {
		t.Errorf("a production run must serve the built-in STUN servers: %s", line)
	}

	// A malformed dev-only override must not stop a production run either.
	if err := ConfigureICE(false, "{not json", "", "", "", "", 0, ""); err != nil {
		t.Errorf("a malformed dev-only override blocked a production run: %v", err)
	}
}

// The warning should name exactly the settings that were ignored.
func TestConfiguredICEOverridesNamesOnlySetSettings(t *testing.T) {
	cases := []struct {
		name                                string
		iceServers, turnURL, user, password string
		want                                string
	}{
		{"nothing set", "", "", "", "", ""},
		{"whitespace is not set", "  ", "\t", " ", "", ""},
		{"credential only", "", "", "", "s3cret", "TURN_CREDENTIAL"},
		{"all set", "[]", "turn:a:1", "u", "p", "ICE_SERVERS, TURN_URL, TURN_USERNAME, TURN_CREDENTIAL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := configuredICEOverrides(tc.iceServers, tc.turnURL, tc.user, tc.password, ""); got != tc.want {
				t.Errorf("configuredICEOverrides() = %q, want %q", got, tc.want)
			}
		})
	}
}

// A stream that reconnects inside the grace window must not be seen as a
// departure: announcing removedUser immediately is what made the remaining
// peers tear down a still-healthy RTCPeerConnection.
func TestReconnectWithinGraceDoesNotAnnounceRemoval(t *testing.T) {
	server := newTestServer(t)
	setRemovalGrace(t, 500*time.Millisecond)

	code := newCode(t)
	alice := connectSSE(t, server, code, "alice")
	defer alice.disconnect()
	bob := connectSSE(t, server, code, "bob")
	alice.expect("newUser")

	// Drop bob's stream and reconnect the same user id before the grace expires.
	bob.disconnect()
	bobAgain := connectSSE(t, server, code, "bob")
	defer bobAgain.disconnect()

	alice.expectSilence(700 * time.Millisecond)
	if got := roomUsers(code); len(got) != 2 {
		t.Errorf("room membership = %v, want alice and bob", got)
	}
}

// Once the grace window passes without a reconnect, the departure is announced.
func TestDepartureAnnouncedAfterGrace(t *testing.T) {
	server := newTestServer(t)
	setRemovalGrace(t, 150*time.Millisecond)

	code := newCode(t)
	alice := connectSSE(t, server, code, "alice")
	defer alice.disconnect()
	bob := connectSSE(t, server, code, "bob")
	alice.expect("newUser")

	bob.disconnect()

	if msg := alice.expect("removedUser"); msg.UserId != "bob" {
		t.Errorf("removedUser userId = %q, want bob", msg.UserId)
	}
	if got := roomUsers(code); len(got) != 1 || got[0] != "alice" {
		t.Errorf("room membership = %v, want [alice]", got)
	}
}

// The renegotiate flag marks an ICE-restart offer. The server decodes into a
// fixed struct and re-marshals, so a field that is not declared is silently
// dropped and the receiver would treat a restart as a duplicate offer.
func TestRenegotiateFlagIsForwarded(t *testing.T) {
	server := newTestServer(t)
	code := newCode(t)
	alice := connectSSE(t, server, code, "alice")
	bob := connectSSE(t, server, code, "bob")
	defer alice.disconnect()
	defer bob.disconnect()
	alice.expect("newUser")

	resp := postEvent(t, server, eventMessage{
		EventType: "newOffer", UserId: "alice", ForUser: "bob", Code: code, Offer: "v=0", Renegotiate: true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("newOffer status = %d, want 200", resp.StatusCode)
	}
	if msg := bob.expect("newOffer"); !msg.Renegotiate {
		t.Errorf("renegotiate flag was dropped while forwarding")
	}
}
