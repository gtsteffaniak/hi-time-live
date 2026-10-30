package routes

// registerConnection records a live SSE stream. Caller must hold connLock.
func registerConnection(conn *connection) {
	connections[conn.connId] = conn
	room, ok := roomConnections[conn.roomId]
	if !ok {
		room = make(map[string]*connection)
		roomConnections[conn.roomId] = room
	}
	room[conn.connId] = conn
}

// unregisterConnection drops a stream and forgets empty room indexes. Caller must hold connLock.
func unregisterConnection(connId, roomId string) {
	delete(connections, connId)
	room, ok := roomConnections[roomId]
	if !ok {
		return
	}
	delete(room, connId)
	if len(room) == 0 {
		delete(roomConnections, roomId)
	}
}

// userHasConnectionLocked reports whether userId has a stream in roomId. Caller must hold connLock.
func userHasConnectionLocked(roomId, userId string) bool {
	for _, conn := range roomConnections[roomId] {
		if conn.userId == userId {
			return true
		}
	}
	return false
}

// connectionRegistryStats returns map sizes for tests.
func connectionRegistryStats() (connectionsLen, roomBucketsLen int) {
	connLock.Lock()
	defer connLock.Unlock()
	return len(connections), len(roomConnections)
}
