package signal

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

func Join(c echo.Context) error {
	code := c.QueryParam("id")
	response := map[string]string{
		"code": code,
	}
	offer := map[string]string{}
	err := json.NewDecoder(c.Request().Body).Decode(&offer)
	if err != nil {
		return err
	}
	err = attemptJoin(code, offer)
	if err != nil {
		response["status"] = "error: unable to join!"
		prettyJSON, _ := json.MarshalIndent(response, "", "  ")
		return c.JSONBlob(500, prettyJSON)
	}
	response["status"] = "ok"
	prettyJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return err
	}
	return c.JSONBlob(http.StatusOK, prettyJSON)
}

func attemptJoin(code string, offer map[string]string) error {
	roomLock.Lock()
	defer roomLock.Unlock()
	if room, ok := rooms[code]; ok {
		room.mu.Lock()
		defer room.mu.Unlock()
		room.users = append(room.users, user{offer: offer})
	} else {
		return fmt.Errorf("room with ID %s does not exists", code)
	}
	return nil
}
