package routes

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// eventTimeout is how long a test waits for an expected server event.
const eventTimeout = 2 * time.Second

func newCode(t *testing.T) string {
	t.Helper()
	return uuid.New().String()
}

func mustJoin(t *testing.T, code, user string) joinResult {
	t.Helper()
	result, err := attemptJoin(code, user)
	if err != nil {
		t.Errorf("attemptJoin(%q, %q) failed: %v", code, user, err)
	}
	return result
}

// newTestServer serves the real routes with templates loaded from the repo.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	templateRenderer = &TemplateRenderer{templateDir: "../templates"}
	if err := templateRenderer.loadTemplates(); err != nil {
		t.Fatalf("could not load templates: %v", err)
	}
	server := httptest.NewServer(NewRouter())
	t.Cleanup(server.Close)
	return server
}

// sseClient is a participant's event stream, as the browser's EventSource sees it.
type sseClient struct {
	t       *testing.T
	userId  string
	roomId  string
	events  chan eventMessage
	cancel  context.CancelFunc
	closed  chan struct{}
	baseURL string
}

// connectSSE opens /events and streams decoded messages into c.events.
func connectSSE(t *testing.T, server *httptest.Server, roomId, userId string) *sseClient {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	c := &sseClient{
		t: t, userId: userId, roomId: roomId,
		events:  make(chan eventMessage, 64),
		cancel:  cancel,
		closed:  make(chan struct{}),
		baseURL: server.URL,
	}
	url := fmt.Sprintf("%s/events?userId=%s&code=%s", server.URL, userId, roomId)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("building SSE request: %v", err)
	}
	// A dedicated transport without keep-alives: the default pooled transport
	// may silently retry an idempotent GET on a fresh socket when a reused
	// connection is stale, which would register a second orphaned SSE stream
	// server-side and defer the user's removal until its heartbeat fails.
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("connecting SSE: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("SSE status = %d, want 200", resp.StatusCode)
	}
	go func() {
		defer close(c.closed)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64<<10), maxEventBody)
		for scanner.Scan() {
			line := scanner.Text()
			payload, found := strings.CutPrefix(line, "data: ")
			if !found {
				continue
			}
			var msg eventMessage
			if err := json.Unmarshal([]byte(payload), &msg); err != nil {
				continue
			}
			select {
			case c.events <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()
	c.expect("acknowledge")
	return c
}

// expect waits for the next event and asserts its type.
func (c *sseClient) expect(eventType string) eventMessage {
	c.t.Helper()
	select {
	case msg := <-c.events:
		if msg.EventType != eventType {
			c.t.Fatalf("%s received %q, want %q", c.userId, msg.EventType, eventType)
		}
		return msg
	case <-time.After(eventTimeout):
		c.t.Fatalf("%s timed out waiting for %q", c.userId, eventType)
		return eventMessage{}
	}
}

// expectSilence asserts that nothing arrives, which is how room isolation is proven.
func (c *sseClient) expectSilence(d time.Duration) {
	c.t.Helper()
	select {
	case msg := <-c.events:
		c.t.Fatalf("%s unexpectedly received %q for user %q", c.userId, msg.EventType, msg.UserId)
	case <-time.After(d):
	}
}

// disconnect closes the stream, as closing a browser tab would.
func (c *sseClient) disconnect() {
	c.cancel()
	<-c.closed
}

func postEvent(t *testing.T, server *httptest.Server, event eventMessage) *http.Response {
	t.Helper()
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshalling event: %v", err)
	}
	resp, err := server.Client().Post(server.URL+"/event", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("posting event: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}
