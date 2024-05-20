package routes

import (
	"encoding/json"
	"fmt"
	"log"
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
		fmt.Println("Deleted " + userId)
	}()
}

func notifyClosedConnection(id string) {
	connLock.Lock()
	defer connLock.Unlock()
	for _, c := range connections {
		c.sendMessage(map[string]string{
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
				fmt.Println("error opening ws ", err)
				return
			}
			var message map[string]string
			err = json.Unmarshal([]byte(msg), &message)
			if err != nil {
				fmt.Println("error for ", connId)
				fmt.Println(err)
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
		fmt.Println("newUser :", conn.userId)
		conn.doNewUserStuff(message)
	case "newOffer":
		fmt.Println("newOffer " + conn.userId)
		notifyNewOffer(message)
	case "answer":
		fmt.Println("answer " + conn.userId)
		notifyNewAnswer(message)
	default:
		fmt.Println("defaulting")
	}
}

func (conn connection) sendMessage(message map[string]string) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("new error %s", err)
	}
	err = websocket.Message.Send(conn.ws, string(jsonData))
	if err != nil {
		return err
	}
	return nil
}

func notifyNewOffer(message map[string]string) error {
	var err error
	connLock.Lock()
	defer connLock.Unlock()
	for _, conn := range connections {
		if conn.userId != message["userId"] {
			fmt.Println("notifyNewOffer " + conn.userId)
			err = conn.sendMessage(map[string]string{
				"eventType":  "newOffer",
				"userId":     message["userId"],
				"offer":      message["offer"],
				"candidates": message["candidates"],
			})
		}
	}
	return err
}

func notifyNewAnswer(message map[string]string) error {
	connLock.Lock()
	defer connLock.Unlock()
	var err error
	for _, conn := range connections {
		if conn.userId != message["forUser"] {
			continue
		}
		fmt.Println("new answer notify " + conn.userId)
		err = conn.sendMessage(map[string]string{
			"eventType":  "answer",
			"userId":     message["userId"],
			"answer":     message["answer"],
			"candidates": message["candidates"],
		})
	}
	return err
}

func (conn connection) doNewUserStuff(message map[string]string) error {
	userId := message["userId"]
	var err error
	numUsers, err := attemptJoin(message["code"], userId)
	if err != nil {
		fmt.Println(err)
		return err
	}
	conn.sendMessage(map[string]string{
		"eventType": "acknowledge",
	})
	if numUsers > 1 {
		notifyNewUser(userId)
	}
	log.Println("new user added : ", userId)
	return err
}

func notifyNewUser(userId string) {
	connLock.Lock()
	defer connLock.Unlock()
	for _, conn := range connections {
		if conn.userId != userId {
			fmt.Println("sending new user notify: ", conn.userId)
			conn.sendMessage(map[string]string{
				"eventType": "newUser",
				"userId":    userId,
			})
		}
	}
}
