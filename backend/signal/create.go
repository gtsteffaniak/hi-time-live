package signal

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func Create(c echo.Context) error {
	code := createRoom()
	response := map[string]string{
		"status": fmt.Sprintf("room with ID %s already exists", code),
	}
	if code != "" {
		response["status"] = "ok"
		response["code"] = code
	}
	prettyJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return err
	}
	return c.JSONBlob(http.StatusOK, prettyJSON)
}

func createRoom() string {
	roomLock.Lock()
	defer roomLock.Unlock()
	code := uuid.New().String()
	if _, ok := rooms[code]; ok {
		return ""
	}
	rooms[code] = room{}
	return code
}
