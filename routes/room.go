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

func attemptJoin(code string, user string) (int, error) {
	if !validCode(code) {
		return 0, fmt.Errorf("could not validate code: %s", code)
	}
	room := getRoom(code)
	room.mu.Lock()
	defer room.mu.Unlock()
	if slices.Contains(room.users, user) {
		return 0, fmt.Errorf("user already exists: %v ", user)
	}
	room.users = append(room.users, user)
	num := len(room.users)
	return num, nil
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

func removeUserFromRoom(code string, id string) {
	r := getRoom(code)
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
		err := templateRenderer.Render(w, "invalidRoom.html", data)
		if err != nil {
			log.Println("could not render invalidRoom.html template", http.StatusInternalServerError)
		}

	} else {
		err := templateRenderer.Render(w, "room.html", data)
		if err != nil {
			log.Println("could not render room.html template", http.StatusInternalServerError)
		}
	}
}
