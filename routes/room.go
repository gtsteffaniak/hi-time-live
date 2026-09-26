package routes

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"slices"
	"sync"
)

type room struct {
	mu    sync.Mutex
	users []string
}

var roomLock sync.Mutex
var rooms = map[string]*room{}

// uuidRegex matches the room codes handed out by the index page.
var uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// getRoom returns the room for a code, creating it if needed.
func getRoom(roomId string) *room {
	roomLock.Lock()
	defer roomLock.Unlock()
	if room, ok := rooms[roomId]; ok {
		return room
	}
	rooms[roomId] = &room{
		users: []string{},
	}
	return rooms[roomId]
}

// joinResult describes what a connection attempt did to the room.
type joinResult struct {
	numUsers  int
	firstJoin bool // false when the same user id is already present (a reconnect)
}

func attemptJoin(code string, user string) (joinResult, error) {
	if !validCode(code) {
		return joinResult{}, fmt.Errorf("could not validate code: %s", code)
	}
	if user == "" {
		return joinResult{}, fmt.Errorf("empty user id")
	}
	room := getRoom(code)
	room.mu.Lock()
	defer room.mu.Unlock()
	if slices.Contains(room.users, user) {
		// A reconnecting client (SSE reconnects on its own) must not be rejected,
		// otherwise it stays in the room list but never gets acknowledged again.
		return joinResult{numUsers: len(room.users), firstJoin: false}, nil
	}
	room.users = append(room.users, user)
	return joinResult{numUsers: len(room.users), firstJoin: true}, nil
}

func validCode(code string) bool {
	return uuidRegex.MatchString(code)
}

// roomUsers returns a copy of the current membership, for tests and diagnostics.
func roomUsers(code string) []string {
	roomLock.Lock()
	r, ok := rooms[code]
	roomLock.Unlock()
	if !ok {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.users)
}

// removeUserFromRoom drops a user and forgets the room once it is empty, so a
// long-running server does not accumulate one entry per room ever created.
func removeUserFromRoom(code string, id string) {
	roomLock.Lock()
	defer roomLock.Unlock()
	r, ok := rooms[code]
	if !ok {
		return
	}
	r.mu.Lock()
	r.users = slices.DeleteFunc(r.users, func(u string) bool { return u == id })
	empty := len(r.users) == 0
	r.mu.Unlock()
	if empty {
		delete(rooms, code)
	}
}

func roomHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	data := map[string]interface{}{}
	data["code"] = id
	data["privacyModal"] = map[string]string{
		"modalType": "privacy",
		"hidden":    "",
		"code":      id,
	}
	if !validCode(id) {
		if err := templateRenderer.RenderWithStatus(w, http.StatusNotFound, "invalidRoom.html", data); err != nil {
			log.Println("could not render invalidRoom.html template:", err)
		}
		return
	}
	if err := templateRenderer.Render(w, "room.html", data); err != nil {
		log.Println("could not render room.html template:", err)
	}
}
