package routes

import (
	"testing"

	"github.com/google/uuid"
)

var numConcurrent = 50

func TestConcurrentCreate(t *testing.T) {
	// Goroutines for creating rooms
	for i := 0; i < numConcurrent; i++ {
		id := uuid.New().String()
		go getRoom(id)
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
	id := uuid.New().String()
	for _, test := range tests {
		for i := 0; i < numConcurrent; i++ {
			go func(offer map[string]string) {
				_, err := attemptJoin(id, uuid.New().String())
				if err != nil {
					t.Errorf("Error joining room: %v", err)
				}
			}(test.offer)
		}
	}

}
