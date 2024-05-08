package routes

import (
	"testing"
)

var numConcurrent = 50

func TestConcurrentCreate(t *testing.T) {
	// Goroutines for creating rooms
	for i := 0; i < numConcurrent; i++ {
		go createRoom()
	}
}

func TestConcurrentJoin(t *testing.T) {
	// Define test data
	tests := []struct {
		offer map[string]string
	}{
		{
			offer: map[string]string{"key1": "value1"},
		},
		{
			offer: map[string]string{"key2": "value2"},
		},
	}
	for _, test := range tests {
		for i := 0; i < numConcurrent; i++ {
			go func(offer map[string]string) {
				roomID := createRoom()
				attemptJoin(roomID, offer)
			}(test.offer)
		}
	}

}
