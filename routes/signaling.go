package routes

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// maxEventBody caps a signaling payload. SDP with bundled candidates is a few
	// KB; anything larger is a bug or an attack.
	maxEventBody = 256 << 10
	// messageBuffer is how far a slow client may fall behind before its messages
	// are dropped instead of stalling the sender.
	messageBuffer = 32
	// heartbeatInterval keeps idle SSE streams alive through proxies and lets the
	// server notice dead sockets.
	heartbeatInterval = 20 * time.Second
)

// removalGrace is how long a user's membership survives their last stream
// closing. EventSource reconnects on its own within a few seconds, and a
// departure announced immediately would make the remaining peers tear down a
// still-healthy RTCPeerConnection. Cancelling the pending removal on reconnect
// keeps the call alive across a transient outage. Guarded by connLock.
var removalGrace = 3 * time.Second

// pendingRemoval is a scheduled departure. The generation identifies the
// schedule its timer belongs to: a reconnect cancels the timer and installs
// nothing, and a later disconnect installs a new generation, so a callback that
// fired while it was waiting for connLock can tell that it no longer owns the
// user's departure. Guarded by connLock.
type pendingRemoval struct {
	timer *time.Timer
	gen   uint64
}

// pendingRemovals holds the scheduled departure for a room+user so a reconnect
// can cancel it. Guarded by connLock.
var pendingRemovals = map[string]*pendingRemoval{}

// removalGen is bumped for every scheduled removal. Guarded by connLock.
var removalGen uint64

func removalKey(roomId, userId string) string {
	return roomId + "\x00" + userId
}

// cancelPendingRemoval stops a scheduled departure. Caller must hold connLock.
func cancelPendingRemoval(roomId, userId string) {
	key := removalKey(roomId, userId)
	if pending, ok := pendingRemovals[key]; ok {
		pending.timer.Stop()
		delete(pendingRemovals, key)
	}
}

// scheduleRemoval announces a departure once the grace window passes, unless the
// user reconnects first.
func scheduleRemoval(roomId, userId string) {
	key := removalKey(roomId, userId)
	connLock.Lock()
	defer connLock.Unlock()
	cancelPendingRemoval(roomId, userId)
	removalGen++
	gen := removalGen
	pending := &pendingRemoval{gen: gen}
	pendingRemovals[key] = pending
	// The generation is captured by value, so the callback never reads state the
	// caller is still writing. connLock is held until the entry above is
	// installed, so no other goroutine sees a half-built schedule either.
	pending.timer = time.AfterFunc(removalGrace, func() { expireRemoval(roomId, userId, gen) })
}

// expireRemoval is the grace timer's callback. It acts only when its generation
// still owns the scheduled removal, so a callback that fired while it was
// waiting for connLock cannot tear down a newer disconnect's grace period.
func expireRemoval(roomId, userId string, gen uint64) {
	key := removalKey(roomId, userId)
	connLock.Lock()
	pending, ok := pendingRemovals[key]
	if !ok || pending.gen != gen {
		connLock.Unlock()
		return
	}
	delete(pendingRemovals, key)
	if userHasConnectionLocked(roomId, userId) {
		connLock.Unlock()
		return
	}
	// connLock stays held across the removal so a reconnect cannot register a
	// live stream between the check above and the removal below. Otherwise the
	// callback could evict a user whose new stream had already registered — and
	// whose announceJoin saw the old membership, so it would not re-add them as
	// a fresh join.
	removeUserFromRoom(roomId, userId)
	connLock.Unlock()
	sendToOthers(roomId, userId, eventMessage{Code: roomId, UserId: userId, EventType: "removedUser"})
	slog.Info("client disconnected", "user", userId, "room", roomId)
}

type connection struct {
	connId    string
	userId    string
	roomId    string
	messageCh chan eventMessage
}

var connections = make(map[string]*connection)
var roomConnections = make(map[string]map[string]*connection)
var connLock sync.Mutex

type eventMessage struct {
	EventType   string `json:"eventType"`
	UserId      string `json:"userId"`
	Offer       string `json:"offer,omitempty"`
	Candidates  string `json:"candidates,omitempty"`
	Candidate   string `json:"candidate,omitempty"`
	MediaState  string `json:"mediaState,omitempty"`
	Code        string `json:"code,omitempty"`
	Message     string `json:"message,omitempty"`
	Answer      string `json:"answer,omitempty"`
	ForUser     string `json:"forUser,omitempty"`
	Time        string `json:"time,omitempty"`
	Renegotiate bool   `json:"renegotiate,omitempty"`
}

// clientEventTypes are the event types a browser is allowed to POST. Anything
// else is rejected rather than forwarded, so the signaling channel cannot be
// used as a generic room-wide message bus.
var clientEventTypes = map[string]bool{
	"newOffer":   true,
	"answer":     true,
	"candidate":  true,
	"mediaState": true,
}

// Handle SSE connection
func sseHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	userId := r.URL.Query().Get("userId")
	roomId := r.URL.Query().Get("code")
	if userId == "" || roomId == "" {
		http.Error(w, "Missing userId or roomId", http.StatusBadRequest)
		return
	}
	if !validCode(roomId) {
		http.Error(w, "Invalid room code", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	conn := &connection{
		connId:    userId + "-" + strings.Split(uuid.New().String(), "-")[0],
		userId:    userId,
		roomId:    roomId,
		messageCh: make(chan eventMessage, messageBuffer),
	}

	connLock.Lock()
	registerConnection(conn)
	// This stream may be replacing one that just closed; keep the membership.
	cancelPendingRemoval(roomId, userId)
	connLock.Unlock()

	defer func() {
		connLock.Lock()
		unregisterConnection(conn.connId, roomId)
		stillPresent := userHasConnectionLocked(roomId, userId)
		connLock.Unlock()
		if stillPresent {
			return
		}
		scheduleRemoval(roomId, userId)
	}()

	if err := writeEvent(w, flusher, eventMessage{EventType: "acknowledge"}); err != nil {
		slog.Error("could not send acknowledgement", "user", userId, "error", err)
		return
	}
	announceJoin(roomId, userId)

	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()
	clientGone := r.Context().Done()
	for {
		select {
		case <-clientGone:
			return
		case <-heartbeat.C:
			if _, err := w.Write([]byte(": keepalive\n\n")); err != nil {
				return
			}
			flusher.Flush()
		case msg := <-conn.messageCh:
			if err := writeEvent(w, flusher, msg); err != nil {
				slog.Error("could not send event", "user", userId, "type", msg.EventType, "error", err)
				return
			}
		}
	}
}

func writeEvent(w http.ResponseWriter, flusher http.Flusher, msg eventMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if _, err = w.Write([]byte("data: " + string(payload) + "\n\n")); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// Handle incoming events
func postEventHandler(w http.ResponseWriter, r *http.Request) {
	var event eventMessage
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxEventBody)).Decode(&event); err != nil {
		http.Error(w, "Error decoding JSON", http.StatusBadRequest)
		return
	}
	if !clientEventTypes[event.EventType] {
		http.Error(w, "Unknown event type", http.StatusBadRequest)
		return
	}
	if !validCode(event.Code) || event.UserId == "" {
		http.Error(w, "Missing or invalid room code or user id", http.StatusBadRequest)
		return
	}
	// The sender must have a live stream in the room it claims to post to, so a
	// stranger cannot inject offers into someone else's call.
	if !hasConnection(event.Code, event.UserId) {
		http.Error(w, "Unknown sender for room", http.StatusForbidden)
		return
	}

	// mediaState is room-scoped presence (mute/video toggles), fanned out to
	// everyone else so a new joiner learns it as soon as it is broadcast.
	if event.EventType == "mediaState" {
		sendToOthers(event.Code, event.UserId, event)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Offers, answers and trickled candidates are all addressed: broadcasting
	// an offer hands it to every other participant, and a second joiner
	// answering an offer meant for someone else corrupts that connection's
	// remote description.
	if event.ForUser == "" {
		http.Error(w, "Missing forUser", http.StatusBadRequest)
		return
	}
	sendMessageToUser(event.Code, event.ForUser, event)
	w.WriteHeader(http.StatusOK)
}

func hasConnection(roomId, userId string) bool {
	connLock.Lock()
	defer connLock.Unlock()
	return userHasConnectionLocked(roomId, userId)
}

// send never blocks: a client that cannot keep up loses messages rather than
// stalling every other participant behind the connection lock.
func (c *connection) send(msg eventMessage) {
	select {
	case c.messageCh <- msg:
	default:
		slog.Warn("dropping event for slow client", "user", c.userId, "type", msg.EventType)
	}
}

// sendToOthers delivers a message to every other participant of one room.
func sendToOthers(roomId, userId string, event eventMessage) {
	connLock.Lock()
	defer connLock.Unlock()
	for _, conn := range roomConnections[roomId] {
		if conn.userId != userId {
			conn.send(event)
		}
	}
}

// sendMessageToUser delivers a message to every stream a user has open in a room.
func sendMessageToUser(roomId, userId string, message eventMessage) {
	connLock.Lock()
	defer connLock.Unlock()
	for _, conn := range roomConnections[roomId] {
		if conn.userId == userId {
			conn.send(message)
		}
	}
}

// announceJoin registers the user with the room and tells the existing
// participants to start negotiating with them.
func announceJoin(roomId, userId string) {
	result, err := attemptJoin(roomId, userId)
	if err != nil {
		slog.Error("error joining room", "room", roomId, "user", userId, "error", err)
		return
	}
	if result.firstJoin && result.numUsers > 1 {
		sendToOthers(roomId, userId, eventMessage{EventType: "newUser", UserId: userId, Code: roomId})
	}
}
