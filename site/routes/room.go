package routes

import (
	"fmt"
	"regexp"
	"sync"
)

type room struct {
	mu    sync.Mutex
	users []string
}

var roomLock sync.Mutex
var rooms = map[string]*room{}

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

func attemptJoin(code string, user string) (*room, error) {
	if !validCode(code) {
		return nil, fmt.Errorf("could not validate code: %s", code)
	}
	fmt.Println("adding user to room")
	room := getRoom(code)
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
	withoutUser := []string{}
	for _, u := range r.users {
		if u != id {
			withoutUser = append(withoutUser, u)
		}
	}
	r.users = withoutUser
}
