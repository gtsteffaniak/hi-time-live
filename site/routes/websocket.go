package routes

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/labstack/echo/v4"
	"golang.org/x/net/websocket"
)

type connection struct {
	room string
	user string
}

var connections = make(map[*websocket.Conn]connection)
var connLock sync.Mutex

func removeConnection(ws *websocket.Conn) {
	defer func() {
		ws.Close()
		connLock.Lock()
		defer connLock.Unlock()

		// Notify all other connections in the same room that the user was removed
		roomCode := connections[ws].room
		userId := connections[ws].user
		for conn, details := range connections {
			if conn == ws || details.room != roomCode {
				continue
			}
			room := rooms[roomCode]
			room.removeUserFromRoom(userId)
			_ = websocket.Message.Send(conn, fmt.Sprintf("User %s removed", userId))
		}

		delete(connections, ws)
	}()

	_ = websocket.Message.Send(ws, "Disconnecting")
}

func wsHandler(c echo.Context) error {
	websocket.Handler(func(ws *websocket.Conn) {
		var connDetails connection
		defer func() {
			removeConnection(ws)

		}()

		connLock.Lock()
		connections[ws] = connDetails
		connLock.Unlock()

		for {
			// Read
			msg := ""
			_ = websocket.Message.Receive(ws, &msg)

			var data map[string]string
			err := json.Unmarshal([]byte(msg), &data)
			if err != nil {
				fmt.Println(err)
				return
			}

			userId := data["user"]
			roomCode := data["code"]
			connDetails.room = roomCode
			connDetails.user = userId

			newUser := user{
				Id:    userId,
				Offer: data["offer"],
			}

			room, err := attemptJoin(roomCode, newUser)
			if err != nil {
				fmt.Println(err)
				return
			}

			jsonData, err := json.Marshal(&room.users)
			if err != nil {
				fmt.Println(err)
				return
			}

			connLock.Lock()
			connections[ws] = connDetails
			for conn := range connections {
				err := websocket.Message.Send(conn, fmt.Sprintf("%v users: %s", len(room.users), jsonData))
				if err != nil {
					c.Logger().Error(err)
				}
			}
			connLock.Unlock()
		}
	}).ServeHTTP(c.Response(), c.Request())
	return nil
}
