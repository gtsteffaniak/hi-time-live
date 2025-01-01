package routes

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func putEventHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("put event!")
	fmt.Fprint(w, "event:put\ndata:put event\n\n")
}

func sseHandler(w http.ResponseWriter, r *http.Request) {
	// Set necessary headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientGone := r.Context().Done()
	rc := http.NewResponseController(w)

	// Log the new connection
	log.Println("New SSE connection established")

	// Set a ticker to send "hello" events every second
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-clientGone:
			log.Println("Client disconnected")
			return
		case <-ticker.C:
			log.Println("New SSE hello")

			// Send a "hello" event
			if _, err := fmt.Fprintf(w, "event:hello\ndata:Hello, world!\n\n"); err != nil {
				log.Printf("Error sending hello event: %s", err.Error())
				return
			}
			rc.Flush() // Ensure the event is immediately sent to the client
		}
	}
}
