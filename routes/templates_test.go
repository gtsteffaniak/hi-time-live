package routes

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// Every page template must render, including the templates it includes. A typo
// in a `{{ template ... }}` name only fails at render time, which is how the
// invalid-room page silently served an empty body.
func TestPageTemplatesRender(t *testing.T) {
	server := newTestServer(t)
	code := newCode(t)

	cases := []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		{"/", http.StatusOK, "Create a new room"},
		{"/room?id=" + code, http.StatusOK, "Privacy Notice"},
		{"/room?id=not-a-uuid", http.StatusNotFound, "not a valid room"},
		{"/room", http.StatusNotFound, "not a valid room"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			resp, err := server.Client().Get(server.URL + tc.path)
			if err != nil {
				t.Fatalf("GET %s: %v", tc.path, err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("reading body: %v", err)
			}
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			if !strings.Contains(string(body), tc.wantBody) {
				t.Errorf("body missing %q, got %d bytes: %.200s", tc.wantBody, len(body), body)
			}
			if !strings.Contains(string(body), "</html>") {
				t.Errorf("template rendering stopped early (no closing html tag): %.200s", body)
			}
		})
	}
}

// The room page embeds the room code into the client scripts; it must be the
// validated code and nothing else.
func TestRoomPageEmbedsOnlyTheValidatedCode(t *testing.T) {
	server := newTestServer(t)
	code := newCode(t)
	resp, err := server.Client().Get(server.URL + "/room?id=" + code)
	if err != nil {
		t.Fatalf("GET /room: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if !strings.Contains(string(body), code) {
		t.Errorf("room page does not contain the room code")
	}
}
