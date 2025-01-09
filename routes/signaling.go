package routes

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type connection struct {
	userId    string
	roomId    string
	messageCh chan eventMessage
}

var connections = make(map[string]*connection)
var connLock sync.Mutex

type eventMessage struct {
	EventType  string `json:"eventType"`
	UserId     string `json:"userId"`
	Offer      string `json:"offer,omitempty"`
	Candidates string `json:"candidates,omitempty"`
	Code       string `json:"code,omitempty"`
	Message    string `json:"message,omitempty"`
	Answer     string `json:"answer,omitempty"`
	Time       string `json:"time,omitempty"`
}

// Handle SSE connection
func sseHandler(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*") // For CORS support

	// Check if the writer supports flushing
	flusher, ok := w.(http.Flusher)
	if !ok {
		// Log the issue for debugging purposes
		fmt.Println("Error: ResponseWriter does not support Flusher")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Generate unique ID for this connection
	userId := r.URL.Query().Get("userId")
	roomId := r.URL.Query().Get("code")
	if userId == "" || roomId == "" {
		http.Error(w, "Missing userId or roomId", http.StatusBadRequest)
		return
	}

	messageCh := make(chan eventMessage, 10)
	connId := userId + "-" + strings.Split(uuid.New().String(), "-")[0]
	conn := &connection{
		roomId:    roomId,
		userId:    userId,
		messageCh: messageCh,
	}

	// Add connection to the map
	connLock.Lock()
	connections[connId] = conn
	connLock.Unlock()

	defer func() {
		connLock.Lock()
		delete(connections, connId)
		connLock.Unlock()
		close(messageCh)
		fmt.Println("Connection closed:", connId)
	}()

	fmt.Println("New connection:", connId)
	// Listen for messages and client disconnection
	clientGone := r.Context().Done()
	// Send an initial ping to confirm the connection
	ack, err := json.Marshal(eventMessage{EventType: "acknowledge"})
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}
	doNewUserStuff(eventMessage{Code: roomId, UserId: userId})
	_, err = fmt.Fprintf(w, "data: %s\n\n", ack)
	if err != nil {
		fmt.Println("Error sending initial ping:", err)
		return
	}
	flusher.Flush()
	for {
		select {
		case <-clientGone:
			fmt.Println("Client disconnected:", connId)
			removeUserMsg := eventMessage{Code: roomId, UserId: userId, EventType: "removeUser"}
			sendToOthers(connId, removeUserMsg)
			return
		case msg := <-messageCh:
			fmt.Println("Sending message to", connId, msg)
			jsonString, err := json.Marshal(msg)
			if err != nil {
				fmt.Println("Error marshalling JSON:", err)
				return
			}
			_, err = fmt.Fprintf(w, "data: %s\n\n", jsonString)
			if err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// Handle incoming events
func postEventHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("new post event")
	var event eventMessage
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Error decoding JSON", http.StatusBadRequest)
		return
	}

	switch event.EventType {
	case "newOffer":
		slog.Debug("newOffer: " + event.UserId)
		sendToOthers(event.UserId, event)
	case "answer":
		slog.Debug("answer: " + event.UserId)
		sendToOthers(event.UserId, event)
	default:
		slog.Debug("Broadcasting to all event type: " + event.EventType)
		broadcastToRoom(event.Code, event)
	}
	w.WriteHeader(http.StatusOK)
}

func sendToOthers(userId string, event eventMessage) {
	connLock.Lock()
	defer connLock.Unlock()

	// Iterate through all active connections
	for _, conn := range connections {
		if conn.userId != userId {
			// Send the message to the connection's channel
			select {
			case conn.messageCh <- event:
				fmt.Println("Sent message to", conn.userId, "about user", userId)
			default:
				fmt.Println("Failed to send message to", conn.userId, "(channel might be full)")
			}
		}
	}
}

// Broadcast message to all connections in the same room
func broadcastToRoom(roomId string, message eventMessage) {
	connLock.Lock()
	defer connLock.Unlock()
	for _, conn := range connections {
		if conn.roomId == roomId {
			conn.messageCh <- message
		}
	}
}

// Send a message to a specific user
func sendMessageToUser(userId string, message eventMessage) {
	connLock.Lock()
	defer connLock.Unlock()
	for _, conn := range connections {
		if conn.userId == userId {
			conn.messageCh <- message
			return
		}
	}
}

func doNewUserStuff(message eventMessage) {
	var err error
	numUsers, err := attemptJoin(message.Code, message.UserId)
	if err != nil {
		slog.Error("attempt join", "error", err)
		return
	}
	sendMessageToUser(message.UserId, eventMessage{EventType: "acknowledge"})
	if numUsers > 1 {
		sendToOthers(message.UserId, eventMessage{EventType: "newUser", UserId: message.UserId})
	}
}
