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
	connId string
	room   string
	userId string
}

var connections = make(map[*websocket.Conn]connection)
var connLock sync.Mutex

func removeConnection(ws *websocket.Conn) {
	fmt.Println("disconnecting")
	defer func() {
		ws.Close()
		connLock.Lock()
		defer connLock.Unlock()
		// Notify all other connections in the same room that the user was removed
		roomCode := connections[ws].room
		userId := connections[ws].userId
		for conn, details := range connections {
			if conn == ws || details.room != roomCode {
				continue
			}
			room := rooms[roomCode]
			room.removeUserFromRoom(userId)
			sendMessage(map[string]string{
				"eventType": "removedUser",
				"userId":    userId,
			}, conn)
		}
		delete(connections, ws)
	}()
}

func wsHandler(c echo.Context) error {
	websocket.Handler(func(ws *websocket.Conn) {
		connId := uuid.New().String()

		defer func() {
			fmt.Println("closing")
			removeConnection(ws)
		}()

		for {
			msg := ""
			err := websocket.Message.Receive(ws, &msg)
			if err != nil {
				fmt.Println("error opening ws ")
				fmt.Println(err)
				return
			}
			var message map[string]string
			err = json.Unmarshal([]byte(msg), &message)
			if err != nil {
				fmt.Println("error for ", connId)
				fmt.Println(err)
				continue
			}
			fmt.Println("new message incoming")
			connLock.Lock()
			connections[ws] = connection{
				connId: connId,
				userId: message["userId"],
				room:   message["code"],
			}
			connLock.Unlock()
			fmt.Println(message["userId"] + " " + connId)
			switch eventType := message["eventType"]; eventType {
			case "offer":
				doNewUserStuff(message)
			case "answer":
				notifyNewAnswer(message)
			case "candidates":
				notifyCandidates(message)
			default:
				fmt.Println("defaulting")
			}
			fmt.Println("new message processing complete")
			if err != nil {
				c.Logger().Error(err)
			}
		}
	}).ServeHTTP(c.Response(), c.Request())
	return nil
}

func sendMessage(message map[string]string, ws *websocket.Conn) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("new error %s", err)
	}
	err = websocket.Message.Send(ws, string(jsonData))
	if err != nil {
		return err
	}
	return nil
}

func notifyNewOffer(user user) error {
	var err error
	connLock.Lock()
	defer connLock.Unlock()
	for ws, conn := range connections {
		if conn.userId == user.Id {
			err = sendMessage(map[string]string{
				"eventType": "acknowledge",
			}, ws)
		} else {
			err = sendMessage(map[string]string{
				"eventType": "newUser",
				"userId":    user.Id,
				"offer":     user.Offer,
			}, ws)
		}
	}
	return err
}

func notifyNewAnswer(message map[string]string) error {
	connLock.Lock()
	defer connLock.Unlock()
	var err error
	for ws, conn := range connections {
		if conn.userId != message["forUser"] {
			continue
		}
		err = sendMessage(map[string]string{
			"eventType":  "answer",
			"userId":     conn.userId,
			"answer":     message["answer"],
			"candidates": "",
		}, ws)
	}
	return err
}

func notifyCandidates(message map[string]string) error {
	connLock.Lock()
	defer connLock.Unlock()
	var err error // Define a variable to store the final error
	for ws, conn := range connections {
		if conn.userId == message["userId"] {
			continue
		}
		err = sendMessage(map[string]string{
			"eventType":  "candidates",
			"userId":     message["userId"],
			"candidates": message["candidates"],
		}, ws)
	}
	return err
}

func doNewUserStuff(message map[string]string) error {
	newUser := user{
		Id:    message["userId"],
		Offer: message["offer"],
	}
	_, err := attemptJoin(message["code"], newUser)
	if err != nil {
		fmt.Println(err)
		return err
	}
	log.Println("new user added : ", message["userId"])
	// notify all websockets on new user connections
	err = notifyNewOffer(newUser)
	return err
}
