package routes

import (
	"fmt"
	"sync"
	"testing"
)

func TestValidCode(t *testing.T) {
	cases := map[string]bool{
		"3fa85f64-5717-4562-b3fc-2c963f66afa6":  true,
		"3FA85F64-5717-4562-B3FC-2C963F66AFA6":  false, // codes are handed out lower case
		"3fa85f64-5717-4562-b3fc-2c963f66afa":   false,
		"3fa85f64-5717-4562-b3fc-2c963f66afa6x": false,
		"../../etc/passwd":                      false,
		"":                                      false,
	}
	for code, want := range cases {
		if got := validCode(code); got != want {
			t.Errorf("validCode(%q) = %v, want %v", code, got, want)
		}
	}
}

func TestAttemptJoinRejectsBadInput(t *testing.T) {
	if _, err := attemptJoin("not-a-uuid", "alice"); err == nil {
		t.Error("expected an error for an invalid room code")
	}
	if _, err := attemptJoin(newCode(t), ""); err == nil {
		t.Error("expected an error for an empty user id")
	}
}

func TestAttemptJoinTreatsRepeatUserAsReconnect(t *testing.T) {
	code := newCode(t)
	first, err := attemptJoin(code, "alice")
	if err != nil || !first.firstJoin || first.numUsers != 1 {
		t.Fatalf("first join = %+v, err = %v", first, err)
	}
	again, err := attemptJoin(code, "alice")
	if err != nil {
		t.Fatalf("a reconnecting user must not be rejected: %v", err)
	}
	if again.firstJoin {
		t.Error("a reconnect must not be reported as a first join")
	}
	if got := roomUsers(code); len(got) != 1 {
		t.Errorf("room membership = %v, want one entry", got)
	}
	// The room map is process-global; leave it empty for churn tests that
	// assert nothing outlives disconnects.
	removeUserFromRoom(code, "alice")
}

func TestRemoveUserForgetsEmptyRooms(t *testing.T) {
	code := newCode(t)
	mustJoin(t, code, "alice")
	mustJoin(t, code, "bob")

	removeUserFromRoom(code, "alice")
	if got := roomUsers(code); len(got) != 1 || got[0] != "bob" {
		t.Fatalf("room membership = %v, want [bob]", got)
	}

	removeUserFromRoom(code, "bob")
	roomLock.Lock()
	_, stillThere := rooms[code]
	roomLock.Unlock()
	if stillThere {
		t.Error("an empty room must be removed so rooms do not leak")
	}
}

func TestRemoveUserFromUnknownRoomDoesNotCreateIt(t *testing.T) {
	code := newCode(t)
	removeUserFromRoom(code, "nobody")
	roomLock.Lock()
	_, created := rooms[code]
	roomLock.Unlock()
	if created {
		t.Error("removing a user must not create the room")
	}
}

// Run with -race: rooms are touched from every SSE goroutine.
func TestConcurrentJoinsAndLeaves(t *testing.T) {
	const rooms, usersPerRoom = 8, 25
	var wg sync.WaitGroup
	codes := make([]string, rooms)
	for i := range codes {
		codes[i] = newCode(t)
	}
	for _, code := range codes {
		for u := 0; u < usersPerRoom; u++ {
			wg.Add(1)
			go func(code string, u int) {
				defer wg.Done()
				user := fmt.Sprintf("user-%d", u)
				mustJoin(t, code, user)
				roomUsers(code)
				removeUserFromRoom(code, user)
			}(code, u)
		}
	}
	wg.Wait()
	for _, code := range codes {
		if got := roomUsers(code); len(got) != 0 {
			t.Errorf("room %s still has %v after everyone left", code, got)
		}
	}
}
