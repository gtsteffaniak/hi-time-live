package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func sseHandler2(w http.ResponseWriter, r *http.Request) {
	// Set HTTP headers required for SSE
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

	// Ticker for periodic events
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	// Channel to handle client disconnection
	clientGone := r.Context().Done()

	for {
		select {
		case <-clientGone:
			fmt.Println("Client disconnected")
			return
		case <-ticker.C:
			respJson, err := json.Marshal(eventMessage{
				Message: "Hello",
				Time:    time.Now().Format(time.RFC3339),
			})
			if err != nil {
				fmt.Printf("Error marshalling JSON: %v\n", err)
				return
			}

			// Send an SSE message
			_, err = fmt.Fprintf(w, "event:hello\ndata:%s\n\n", respJson)
			if err != nil {
				fmt.Printf("Error writing to client: %v\n", err)
				return
			}
			// Flush the data to the client
			flusher.Flush()
		}
	}
}
