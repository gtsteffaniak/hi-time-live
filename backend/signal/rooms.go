package signal

import (
	"sync"
)

type room struct {
	users []user
	mu    sync.RWMutex
}

type user struct {
	offer map[string]string
}

var roomLock = sync.RWMutex{}
var rooms = map[string]room{}
