package routes

import (
	"fmt"
	"regexp"
	"sync"
)

type room struct {
	users []user
	mu    sync.Mutex
}

type user struct {
	Id    string `json:"id"`
	Offer string `json:"offer"`
}

var roomLock sync.Mutex
var rooms = map[string]*room{}

func createRoom(code string) {
	roomLock.Lock()
	defer roomLock.Unlock()
	if _, ok := rooms[code]; !ok {
		rooms[code] = &room{} // Change room{} to &room{}
	}
}

func attemptJoin(code string, user user) (*room, error) {
	if !validCode(code) {
		return nil, fmt.Errorf("could not validate code: %s", code)
	}
	roomLock.Lock()
	room, ok := rooms[code]
	roomLock.Unlock()
	if !ok {
		createRoom(code)
		roomLock.Lock() // Lock again to prevent race conditions
		room = rooms[code]
		roomLock.Unlock()
	}
	room.mu.Lock()
	defer room.mu.Unlock()
	room.users = append(room.users, user)
	return room, nil
}

func validCode(code string) bool {
	// Regular expression to match UUID format
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	valid := uuidRegex.MatchString(code)
	if !valid {
		fmt.Printf("invalid code: '%s'", code)
	}
	// Check if the code matches the UUID format
	return valid
}

func (r *room) removeUserFromRoom(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, u := range r.users {
		if u.Id == id {
			// Remove the user from the slice
			r.users = append(r.users[:i], r.users[i+1:]...)
			return // Exit the loop once the user is removed
		}
	}
}
