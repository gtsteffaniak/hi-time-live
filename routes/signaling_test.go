package routes

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSSERequiresUserAndValidRoom(t *testing.T) {
	server := newTestServer(t)
	cases := map[string]string{
		"/events":                               "missing both",
		"/events?userId=alice":                  "missing room",
		"/events?code=" + newCode(t):            "missing user",
		"/events?userId=alice&code=not-a-uuid":  "invalid room code",
		"/events?userId=alice&code=../../etc/p": "path-like room code",
	}
	for path, name := range cases {
		resp, err := server.Client().Get(server.URL + path)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, resp.StatusCode)
		}
	}
}

func TestExistingParticipantsLearnAboutANewUser(t *testing.T) {
	server := newTestServer(t)
	code := newCode(t)

	alice := connectSSE(t, server, code, "alice")
	bob := connectSSE(t, server, code, "bob")
	defer alice.disconnect()
	defer bob.disconnect()

	msg := alice.expect("newUser")
	if msg.UserId != "bob" {
		t.Errorf("newUser userId = %q, want bob", msg.UserId)
	}
	// The joiner must not be told about itself.
	bob.expectSilence(200 * time.Millisecond)
}

func TestRoomsAreIsolated(t *testing.T) {
	server := newTestServer(t)
	roomA, roomB := newCode(t), newCode(t)

	alice := connectSSE(t, server, roomA, "alice")
	mallory := connectSSE(t, server, roomB, "mallory")
	defer alice.disconnect()
	defer mallory.disconnect()

	// Joining room B must not reach room A.
	alice.expectSilence(200 * time.Millisecond)

	bob := connectSSE(t, server, roomA, "bob")
	defer bob.disconnect()
	alice.expect("newUser")

	// Offers posted in room A must not leak into room B.
	resp := postEvent(t, server, eventMessage{EventType: "newOffer", UserId: "bob", Code: roomA, ForUser: "alice", Offer: "v=0"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("newOffer status = %d, want 200", resp.StatusCode)
	}
	alice.expect("newOffer")
	mallory.expectSilence(200 * time.Millisecond)
}

func TestAnswerIsDeliveredOnlyToItsTarget(t *testing.T) {
	server := newTestServer(t)
	code := newCode(t)

	alice := connectSSE(t, server, code, "alice")
	bob := connectSSE(t, server, code, "bob")
	carol := connectSSE(t, server, code, "carol")
	defer alice.disconnect()
	defer bob.disconnect()
	defer carol.disconnect()

	alice.expect("newUser") // bob
	alice.expect("newUser") // carol
	bob.expect("newUser")   // carol

	postEvent(t, server, eventMessage{EventType: "answer", UserId: "bob", ForUser: "alice", Code: code, Answer: "v=0"})

	if msg := alice.expect("answer"); msg.UserId != "bob" {
		t.Errorf("answer came from %q, want bob", msg.UserId)
	}
	carol.expectSilence(200 * time.Millisecond)
}

func TestOfferIsDeliveredOnlyToItsTarget(t *testing.T) {
	server := newTestServer(t)
	code := newCode(t)

	alice := connectSSE(t, server, code, "alice")
	bob := connectSSE(t, server, code, "bob")
	carol := connectSSE(t, server, code, "carol")
	defer alice.disconnect()
	defer bob.disconnect()
	defer carol.disconnect()

	alice.expect("newUser")
	alice.expect("newUser")
	bob.expect("newUser")

	// An offer aimed at carol must never reach bob: if a bystander answered an
	// offer meant for someone else, its answer would poison the intended
	// connection's remote description.
	resp := postEvent(t, server, eventMessage{EventType: "newOffer", UserId: "alice", ForUser: "carol", Code: code, Offer: "v=0"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("newOffer status = %d, want 200", resp.StatusCode)
	}
	if msg := carol.expect("newOffer"); msg.UserId != "alice" {
		t.Errorf("offer came from %q, want alice", msg.UserId)
	}
	bob.expectSilence(200 * time.Millisecond)
}

func TestPostEventRejectsMalformedAndUnauthorisedPayloads(t *testing.T) {
	server := newTestServer(t)
	code := newCode(t)
	alice := connectSSE(t, server, code, "alice")
	defer alice.disconnect()

	cases := []struct {
		name  string
		event eventMessage
		want  int
	}{
		{"unknown event type", eventMessage{EventType: "shutdown", UserId: "alice", Code: code}, http.StatusBadRequest},
		{"missing event type", eventMessage{UserId: "alice", Code: code}, http.StatusBadRequest},
		{"invalid room code", eventMessage{EventType: "newOffer", UserId: "alice", Code: "nope"}, http.StatusBadRequest},
		{"missing user", eventMessage{EventType: "newOffer", Code: code}, http.StatusBadRequest},
		{"offer without target", eventMessage{EventType: "newOffer", UserId: "alice", Code: code}, http.StatusBadRequest},
		{"answer without target", eventMessage{EventType: "answer", UserId: "alice", Code: code}, http.StatusBadRequest},
		{"sender not in room", eventMessage{EventType: "newOffer", UserId: "stranger", ForUser: "alice", Code: code}, http.StatusForbidden},
		{"sender in another room", eventMessage{EventType: "newOffer", UserId: "alice", ForUser: "bob", Code: newCode(t)}, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := postEvent(t, server, tc.event).StatusCode; got != tc.want {
				t.Errorf("status = %d, want %d", got, tc.want)
			}
		})
	}
	alice.expectSilence(200 * time.Millisecond)
}

func TestOversizedPayloadIsRejected(t *testing.T) {
	server := newTestServer(t)
	code := newCode(t)
	alice := connectSSE(t, server, code, "alice")
	defer alice.disconnect()

	huge := eventMessage{EventType: "newOffer", UserId: "alice", Code: code, Offer: strings.Repeat("a", maxEventBody+1)}
	if got := postEvent(t, server, huge).StatusCode; got != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", got)
	}
}

func TestDisconnectAnnouncesRemovalAndFreesTheRoom(t *testing.T) {
	server := newTestServer(t)
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

func TestSecondTabDoesNotEvictTheFirst(t *testing.T) {
	server := newTestServer(t)
	code := newCode(t)

	alice := connectSSE(t, server, code, "alice")
	defer alice.disconnect()
	bobTab1 := connectSSE(t, server, code, "bob")
	defer bobTab1.disconnect()
	alice.expect("newUser")

	bobTab2 := connectSSE(t, server, code, "bob")
	bobTab2.disconnect()

	// Closing the duplicate stream must not evict bob from the room.
	alice.expectSilence(300 * time.Millisecond)
	if got := roomUsers(code); len(got) != 2 {
		t.Errorf("room membership = %v, want alice and bob", got)
	}
}

// A client that stops reading must not stall delivery to everyone else.
func TestSlowClientDoesNotBlockTheRoom(t *testing.T) {
	server := newTestServer(t)
	code := newCode(t)

	alice := connectSSE(t, server, code, "alice")
	defer alice.disconnect()
	bob := connectSSE(t, server, code, "bob")
	defer bob.disconnect()
	alice.expect("newUser")

	// Simulate a stalled consumer by filling its outbound buffer directly.
	connLock.Lock()
	for _, conn := range connections {
		if conn.userId == "bob" {
			for i := 0; i < messageBuffer*3; i++ {
				conn.send(eventMessage{EventType: "newOffer", UserId: "filler"})
			}
		}
	}
	connLock.Unlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		postEvent(t, server, eventMessage{EventType: "answer", UserId: "bob", ForUser: "alice", Code: code, Answer: "v=0"})
	}()
	select {
	case <-done:
	case <-time.After(eventTimeout):
		t.Fatal("delivery blocked behind a slow client")
	}
	alice.expect("answer")
}

func TestCandidateIsDeliveredOnlyToItsTarget(t *testing.T) {
	server := newTestServer(t)
	code := newCode(t)

	alice := connectSSE(t, server, code, "alice")
	bob := connectSSE(t, server, code, "bob")
	carol := connectSSE(t, server, code, "carol")
	defer alice.disconnect()
	defer bob.disconnect()
	defer carol.disconnect()

	alice.expect("newUser")
	alice.expect("newUser")
	bob.expect("newUser")

	resp := postEvent(t, server, eventMessage{EventType: "candidate", UserId: "alice", ForUser: "bob", Code: code, Candidate: `{"candidate":"c1"}`})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("candidate status = %d, want 200", resp.StatusCode)
	}
	if msg := bob.expect("candidate"); msg.UserId != "alice" {
		t.Errorf("candidate came from %q, want alice", msg.UserId)
	}
	carol.expectSilence(200 * time.Millisecond)

	if got := postEvent(t, server, eventMessage{EventType: "candidate", UserId: "alice", Code: code, Candidate: `{"candidate":"c1"}`}).StatusCode; got != http.StatusBadRequest {
		t.Errorf("candidate without forUser status = %d, want 400", got)
	}
}

// mediaState is room-scoped presence: it goes to everyone but the sender and
// needs no forUser.
func TestMediaStateBroadcastsToOthers(t *testing.T) {
	server := newTestServer(t)
	code := newCode(t)

	alice := connectSSE(t, server, code, "alice")
	bob := connectSSE(t, server, code, "bob")
	defer alice.disconnect()
	defer bob.disconnect()
	alice.expect("newUser")

	resp := postEvent(t, server, eventMessage{EventType: "mediaState", UserId: "bob", Code: code, MediaState: `{"audio":false,"video":true}`})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mediaState status = %d, want 200", resp.StatusCode)
	}
	if msg := alice.expect("mediaState"); msg.UserId != "bob" {
		t.Errorf("mediaState came from %q, want bob", msg.UserId)
	}
	bob.expectSilence(200 * time.Millisecond)
}
