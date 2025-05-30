package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// connection represents an SSE connection for a user in a room.
type connection struct {
	userId         string // The actual ID of the user (e.g., "G__3804d93a")
	roomId         string // The ID of the room the user is in
	connInstanceId string // A unique ID for this specific SSE connection instance
	messageCh      chan eventMessage
}

// connections stores all active SSE connection instances.
// The key is connInstanceId.
var connections = make(map[string]*connection)
var connLock sync.Mutex

// eventMessage defines the structure for messages exchanged via SSE.
type eventMessage struct {
	EventType  string `json:"eventType"`
	UserId     string `json:"userId"`               // ID of the user sending the event or concerned by it
	Offer      string `json:"offer,omitempty"`      // SDP offer
	Answer     string `json:"answer,omitempty"`     // SDP answer
	Candidates string `json:"candidates,omitempty"` // JSON string of ICE candidates
	Code       string `json:"code,omitempty"`       // Room code/ID
	Message    string `json:"message,omitempty"`    // Generic message content
	ForUser    string `json:"forUser,omitempty"`    // Specifies the recipient user ID for direct messages like offer/answer
	Time       string `json:"time,omitempty"`       // Timestamp, if needed
}

// sseHandler handles new Server-Sent Event (SSE) connections.
// Each user joining a room establishes an SSE connection.
func sseHandler(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*") // For CORS

	flusher, ok := w.(http.Flusher)
	if !ok {
		fmt.Println("Error: ResponseWriter does not support Flusher")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	userId := r.URL.Query().Get("userId")
	roomId := r.URL.Query().Get("code")
	if userId == "" || roomId == "" {
		http.Error(w, "Missing userId or roomId", http.StatusBadRequest)
		return
	}

	// Create a unique ID for this specific connection instance
	connInstanceId := userId + "-" + strings.Split(uuid.New().String(), "-")[0]
	messageCh := make(chan eventMessage, 10) // Buffered channel for this connection

	conn := &connection{
		userId:         userId,
		roomId:         roomId,
		connInstanceId: connInstanceId,
		messageCh:      messageCh,
	}

	// Add connection to the map
	connLock.Lock()
	connections[connInstanceId] = conn
	connLock.Unlock()
	fmt.Printf("SSE connection established: userId=%s, roomId=%s, connInstanceId=%s\n", userId, roomId, connInstanceId)

	// Defer cleanup when the client disconnects or function exits
	defer func(uid string, rid string, cInstId string, mCh chan eventMessage) {
		connLock.Lock()
		delete(connections, cInstId)
		connLock.Unlock()
		close(mCh) // Close the channel for this connection

		fmt.Printf("Client disconnected: userId=%s, roomId=%s, connInstanceId=%s\n", uid, rid, cInstId)

		// TODO: Implement removeUserFromRoom(roomId, userId) if you maintain a separate list of users per room.
		// removeUserFromRoom(rid, uid)

		// Notify other users in the same room that this user has left
		removedUserMsg := eventMessage{
			Code:      rid, // Room ID
			UserId:    uid, // ID of the user who left
			EventType: "removedUser",
		}
		sendToOthersInRoomExcludingUser(rid, uid, removedUserMsg)
	}(userId, roomId, connInstanceId, messageCh)

	// Perform actions for a new user (join room, notify others)
	// This is called after the connection is registered and defer is set.
	doNewUserStuff(eventMessage{Code: roomId, UserId: userId})

	// Send an initial acknowledge message to the client
	ackMsg := eventMessage{EventType: "acknowledge", UserId: userId}
	ackBytes, err := json.Marshal(ackMsg)
	if err != nil {
		fmt.Println("Error marshalling acknowledge JSON for user", userId, ":", err)
		return // Exit if we can't even send an ack
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", ackBytes)
	if err != nil {
		fmt.Println("Error sending initial acknowledge to user", userId, ":", err)
		return // Exit if send fails
	}
	flusher.Flush()
	fmt.Println("Sent acknowledge to user:", userId)

	clientGone := r.Context().Done() // Channel to detect client disconnection
	for {
		select {
		case <-clientGone:
			// Client disconnected, the defer function will handle cleanup and notification.
			fmt.Println("Client gone detected for userId:", userId, "connInstanceId:", connInstanceId)
			return // Exit the loop, which will trigger the defer.
		case msg := <-messageCh:
			// Send message from channel to the client
			jsonString, err := json.Marshal(msg)
			if err != nil {
				fmt.Println("Error marshalling JSON for SSE message to user", userId, ":", err)
				continue // Skip this message if marshalling fails
			}
			_, err = fmt.Fprintf(w, "data: %s\n\n", jsonString)
			if err != nil {
				fmt.Println("Error sending SSE message to user", userId, ":", err)
				// If sending fails, it might mean the client has already disconnected.
				// The clientGone channel should eventually detect this.
				// Consider breaking or returning if Fprintf fails repeatedly.
				return // Exit, defer will handle cleanup
			}
			flusher.Flush()
			// fmt.Println("Sent message to userId:", userId, "eventType:", msg.EventType)
		}
	}
}

// postEventHandler handles incoming WebRTC signaling events (offers, answers, candidates) via POST requests.
func postEventHandler(w http.ResponseWriter, r *http.Request) {
	var event eventMessage
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Error decoding JSON", http.StatusBadRequest)
		return
	}

	// All signaling messages like offer/answer should be targeted using ForUser.
	if event.ForUser == "" && (event.EventType == "newOffer" || event.EventType == "answer") {
		errMsg := fmt.Sprintf("Error: Event '%s' received without ForUser field. From UserId: %s", event.EventType, event.UserId)
		fmt.Println(errMsg)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	switch event.EventType {
	case "newOffer":
		fmt.Printf("Received newOffer from %s FOR %s\n", event.UserId, event.ForUser)
		sendMessageToUser(event.ForUser, event) // Send offer only to the target user
	case "answer":
		fmt.Printf("Received answer from %s FOR %s\n", event.UserId, event.ForUser)
		sendMessageToUser(event.ForUser, event) // Send answer only to the original offerer
	// Note: ICE candidates are now typically bundled with offer/answer in this setup.
	// If you were to send candidates separately, they would also need a ForUser field.
	default:
		fmt.Println("Unknown event type in POST: ", event.EventType, "from User:", event.UserId, "ForUser:", event.ForUser)
		http.Error(w, "Unknown or unhandled event type", http.StatusBadRequest)
	}
	w.WriteHeader(http.StatusOK)
}

// sendMessageToUser sends a message to all connection instances of a specific userId.
func sendMessageToUser(targetUserId string, message eventMessage) {
	connLock.Lock()
	defer connLock.Unlock()

	sent := false
	for _, conn := range connections {
		if conn.userId == targetUserId {
			select {
			case conn.messageCh <- message:
				sent = true
			default:
				// This might happen if the user's message channel is full.
				fmt.Printf("  Failed to queue message type '%s' for userId: %s (channel full or closed for connInstanceId: %s)\n", message.EventType, targetUserId, conn.connInstanceId)
			}
		}
	}
	if !sent {
		fmt.Printf("  Warning: No active connection found for userId: %s to send message type '%s'\n", targetUserId, message.EventType)
	}
}

// sendToOthersInRoomExcludingUser sends a message to all users in a specific room, excluding a given userId.
func sendToOthersInRoomExcludingUser(roomId string, excludeUserId string, message eventMessage) {
	connLock.Lock()
	defer connLock.Unlock()

	fmt.Printf("Broadcasting event '%s' to room '%s', excluding user '%s'\n", message.EventType, roomId, excludeUserId)
	sentCount := 0
	for _, conn := range connections {
		if conn.roomId == roomId && conn.userId != excludeUserId {
			select {
			case conn.messageCh <- message:
				// fmt.Printf("  Queued broadcast message for userId: %s (connInstanceId: %s)\n", conn.userId, conn.connInstanceId)
				sentCount++
			default:
				fmt.Printf("  Failed to queue broadcast message for userId: %s (channel full or closed for connInstanceId: %s)\n", conn.userId, conn.connInstanceId)
			}
		}
	}
	fmt.Printf("Broadcast event '%s' queued for %d users in room '%s'.\n", message.EventType, sentCount, roomId)
}

// doNewUserStuff handles logic when a new user's SSE connection is established.
func doNewUserStuff(newUserMsg eventMessage) { // newUserMsg contains Code (roomId) and UserId (of new user)
	roomId := newUserMsg.Code
	newUserId := newUserMsg.UserId

	// TODO: Implement attemptJoin(roomId, userId) if you need to track users in rooms.
	// This function would typically add the user to a list for the room and return the number of users.
	// For this example, we'll simulate it.
	// numUsers, err := attemptJoin(roomId, newUserId)
	// if err != nil {
	// 	fmt.Println("Error joining room for user", newUserId, ":", err)
	// 	return
	// }
	// fmt.Printf("User %s joined room %s. Room user count: %d (simulated)\n", newUserId, roomId, numUsers)

	// The acknowledge message is now sent from sseHandler after doNewUserStuff.

	// Notify other users in the same room about the new user.
	// The UserId in the event is the ID of the user who just joined.
	// This message will prompt existing clients to initiate WebRTC connections with the new user.
	eventToSend := eventMessage{
		EventType: "newUser", // Event type indicating a new user has joined
		UserId:    newUserId, // The ID of the new user
		Code:      roomId,    // The room ID they joined
	}
	sendToOthersInRoomExcludingUser(roomId, newUserId, eventToSend)
}
