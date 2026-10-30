package routes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// roomPage fetches the rendered room page, which embeds the client scripts.
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

// renderedIceServers returns the JSON the client reads its ICE config from, so
// assertions are not confused by the script's own fallback STUN list.
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

// A configured TURN server must reach the browser, otherwise peers behind
// symmetric NAT have no relay candidate and can never connect.
func TestRoomPageRendersConfiguredTurnServer(t *testing.T) {
	server := newTestServer(t)
	if err := ConfigureICE(true, "", "turn:turn.example.com:3478", "alice", "s3cret"); err != nil {
		t.Fatalf("ConfigureICE: %v", err)
	}
	line := renderedIceServers(t, roomPage(t, server, server.URL+"/room?id="+newCode(t)))

	for _, want := range []string{"turn:turn.example.com:3478", `"username":"alice"`, `"credential":"s3cret"`} {
		if !strings.Contains(line, want) {
			t.Errorf("rendered ICE servers missing %q: %s", want, line)
		}
	}
	// The STUN defaults must survive alongside the TURN entry.
	if !strings.Contains(line, "stun:stun.l.google.com:19302") {
		t.Errorf("rendered ICE servers dropped the default STUN servers: %s", line)
	}
}

// ICE_SERVERS is a full override, so operators can express anything the
// browser's RTCIceServer supports without a code change.
func TestICEServersOverrideReplacesDefaults(t *testing.T) {
	server := newTestServer(t)
	override := `[{"urls":["stun:custom.example.com:3478","turns:turn.example.com:5349"],"username":"u","credential":"p"}]`
	if err := ConfigureICE(true, override, "turn:ignored.example.com:3478", "ignored", "ignored"); err != nil {
		t.Fatalf("ConfigureICE: %v", err)
	}
	line := renderedIceServers(t, roomPage(t, server, server.URL+"/room?id="+newCode(t)))

	if !strings.Contains(line, "stun:custom.example.com:3478") || !strings.Contains(line, "turns:turn.example.com:5349") {
		t.Errorf("ICE_SERVERS override not rendered: %s", line)
	}
	if strings.Contains(line, "stun:stun.l.google.com:19302") {
		t.Errorf("override should replace the defaults, but they are still present: %s", line)
	}
	if strings.Contains(line, "turn:ignored.example.com:3478") {
		t.Errorf("TURN flags must be ignored when ICE_SERVERS is set: %s", line)
	}
}

// An override that parses but carries no usable URL must be rejected at startup
// rather than shipped to the browser, where the WebRTC algorithm would refuse
// the whole configuration and every room would fail to connect.
func TestInvalidICEServersIsRejected(t *testing.T) {
	cases := map[string]string{
		"malformed JSON":     "{not json",
		"missing urls":       `[{}]`,
		"empty url list":     `[{"urls":[]}]`,
		"blank url string":   `[{"urls":""}]`,
		"blank url in array": `[{"urls":["  "]}]`,
		"non-string url":     `[{"urls":[42]}]`,
		"later entry empty":  `[{"urls":"stun:ok.example.com:3478"},{}]`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if err := ConfigureICE(true, raw, "", "", ""); err == nil {
				t.Fatalf("ConfigureICE accepted invalid ICE_SERVERS: %s", raw)
			}
		})
	}
}

// urls may be a bare string as well as an array; both are valid RTCIceServer.
func TestICEServersAcceptsSingleURLString(t *testing.T) {
	t.Cleanup(func() { iceServersJSON = "" })
	if err := ConfigureICE(true, `[{"urls":"stun:only.example.com:3478"}]`, "", "", ""); err != nil {
		t.Fatalf("ConfigureICE rejected a single-string urls value: %v", err)
	}
	if !strings.Contains(string(iceServersJSON), "stun:only.example.com:3478") {
		t.Errorf("single-string urls value was not rendered: %s", iceServersJSON)
	}
}

// The overrides are a development-only feature: a production run must never
// publish a relay credential on a room page, because anyone who can load any
// room URL can read it and reuse the relay outside this app.
func TestICEOverridesAreIgnoredOutsideDevMode(t *testing.T) {
	server := newTestServer(t)
	err := ConfigureICE(false,
		`[{"urls":"turn:relay.example.com:3478","username":"alice","credential":"s3cret"}]`,
		"turn:relay.example.com:3478", "alice", "s3cret")
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
	if err := ConfigureICE(false, "{not json", "", "", ""); err != nil {
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
			if got := configuredICEOverrides(tc.iceServers, tc.turnURL, tc.user, tc.password); got != tc.want {
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
