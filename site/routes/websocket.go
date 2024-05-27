package routes

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/net/websocket"
)

type connection struct {
	ws     *websocket.Conn
	id     string
	roomId string
	userId string
}

var connections = make(map[string]connection)
var connLock sync.Mutex

func (conn connection) close() {
	defer func() {
		conn.ws.Close()
		connLock.Lock()
		room := getRoom(conn.roomId)
		userId := conn.userId
		room.removeUserFromRoom(userId)
		delete(connections, conn.id)
		connLock.Unlock()
		notifyClosedConnection(userId)
		slog.Debug("Deleted " + userId)
	}()
}

func notifyClosedConnection(id string) {
	connLock.Lock()
	defer connLock.Unlock()
	for _, c := range connections {
		_ = c.sendMessage(map[string]string{
			"eventType": "removedUser",
			"userId":    id,
		})
	}
}

func wsHandler(c echo.Context) error {
	websocket.Handler(func(ws *websocket.Conn) {
		connId := uuid.New().String()
		newConnection := connection{
			id: connId,
			ws: ws,
		}
		defer func() {
			newConnection.close()
		}()

		for {
			msg := ""
			err := websocket.Message.Receive(ws, &msg)
			if err != nil {
				slog.Error("Incoming websocket failed ", "connId", connId, "error", err)
				return
			}
			var message map[string]string
			err = json.Unmarshal([]byte(msg), &message)
			if err != nil {
				slog.Error("Unable to handle message", "connId", connId, "error", err)
				continue
			}
			newConnection.userId = message["userId"]
			newConnection.roomId = message["code"]
			connLock.Lock()
			connections[connId] = newConnection
			connLock.Unlock()
			newConnection.eventRouter(message)
		}
	}).ServeHTTP(c.Response(), c.Request())
	return nil
}

func (conn connection) eventRouter(message map[string]string) {
	switch eventType := message["eventType"]; eventType {
	case "newUser":
		slog.Debug("newUser :" + conn.userId)
		conn.doNewUserStuff(message)
	case "newOffer":
		slog.Debug("newOffer " + conn.userId)
		notifyNewOffer(message)
	case "answer":
		slog.Debug("answer " + conn.userId)
		notifyNewAnswer(message)
	default:
		slog.Debug("defaulting")
	}
}

func (conn connection) sendMessage(message map[string]string) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}
	err = websocket.Message.Send(conn.ws, string(jsonData))
	if err != nil {
		return err
	}
	return nil
}

func notifyNewOffer(message map[string]string) {
	connLock.Lock()
	defer connLock.Unlock()
	for _, conn := range connections {
		if conn.userId != message["userId"] {
			slog.Debug("notifyNewOffer ", "connId", conn.id, "userId", conn.userId)
			_ = conn.sendMessage(map[string]string{
				"eventType":  "newOffer",
				"userId":     message["userId"],
				"offer":      message["offer"],
				"candidates": message["candidates"],
			})
		}
	}
}

func notifyNewAnswer(message map[string]string) {
	connLock.Lock()
	defer connLock.Unlock()
	for _, conn := range connections {
		if conn.userId != message["forUser"] {
			continue
		}
		slog.Debug("new answer notify ", "connId", conn.id, "userId", conn.userId)
		_ = conn.sendMessage(map[string]string{
			"eventType":  "answer",
			"userId":     message["userId"],
			"answer":     message["answer"],
			"candidates": message["candidates"],
		})
	}
}

func (conn connection) doNewUserStuff(message map[string]string) {
	userId := message["userId"]
	var err error
	numUsers, err := attemptJoin(message["code"], userId)
	if err != nil {
		slog.Error("attempt join", "error", err)
		return
	}
	_ = conn.sendMessage(map[string]string{
		"eventType": "acknowledge",
	})
	if numUsers > 1 {
		notifyNewUser(userId)
	}
	slog.Debug("new user added", "connId", conn.id, "userId", userId)
}

func notifyNewUser(userId string) {
	connLock.Lock()
	defer connLock.Unlock()
	for _, conn := range connections {
		if conn.userId != userId {
			slog.Debug("sending new user notify", conn.id, "userId", userId)
			_ = conn.sendMessage(map[string]string{
				"eventType": "newUser",
				"userId":    userId,
			})
		}
	}
}
