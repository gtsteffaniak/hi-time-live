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
			message := map[string]any{
				"eventType": "removedUser",
				"userId":    userId,
			}
			fmt.Println("disconnected ", userId)
			jsonData, _ := json.Marshal(message)
			_ = websocket.Message.Send(conn, string(jsonData))
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

func notifyNewOffer(user user) error {
	connLock.Lock()
	defer connLock.Unlock()
	for ws, conn := range connections {
		if conn.userId == user.Id {
			return nil
		}
		message := map[string]string{
			"eventType":  "newUser",
			"userId":     user.Id,
			"offer":      user.Offer,
			"candidates": "",
		}
		jsonData, err := json.Marshal(message)
		if err != nil {
			return fmt.Errorf("new error %s", err)
		}
		err = websocket.Message.Send(ws, string(jsonData))
		if err != nil {
			return err
		}
	}
	return nil
}

func notifyNewAnswer(message map[string]string) error {
	connLock.Lock()
	defer connLock.Unlock()
	var finalErr error // Define a variable to store the final error
	for ws, conn := range connections {
		if conn.userId != message["forUser"] {
			continue
		}
		message := map[string]string{
			"eventType":  "answer",
			"userId":     conn.userId,
			"answer":     message["answer"],
			"candidates": "",
		}
		jsonData, err := json.Marshal(message)
		if err != nil {
			return fmt.Errorf("new error %s", err)
		}
		finalErr = websocket.Message.Send(ws, string(jsonData))
	}
	return finalErr
}
func notifyCandidates(message map[string]string) error {
	connLock.Lock()
	defer connLock.Unlock()
	var finalErr error // Define a variable to store the final error
	for ws, conn := range connections {
		if conn.userId == message["userId"] {
			continue
		}
		message := map[string]string{
			"eventType":  "candidates",
			"userId":     message["userId"],
			"candidates": message["candidates"],
		}
		jsonData, err := json.Marshal(message)
		if err != nil {
			return fmt.Errorf("new error %s", err)
		}
		finalErr = websocket.Message.Send(ws, string(jsonData))
	}
	return finalErr
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

func withoutCurrentUser(id string, users []user) []user {
	var filteredUsers []user
	for _, u := range users {
		if u.Id != id {
			filteredUsers = append(filteredUsers, u)
		}
	}
	return filteredUsers
}
