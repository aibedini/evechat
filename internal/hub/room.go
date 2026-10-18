package hub

// Room is the subscriber set of one conversation: visitor sockets, the assigned
// agent's sockets and any admin watching the thread.
//
// Rooms are only mutated from the hub goroutine, so no lock is needed.
type Room struct {
	convID  string
	clients map[string]*Client
}

func newRoom(convID string) *Room {
	return &Room{convID: convID, clients: make(map[string]*Client, 4)}
}

func (r *Room) add(c *Client) {
	if c == nil {
		return
	}
	r.clients[c.ConnID] = c
	c.ConvIDs[r.convID] = struct{}{}
}

func (r *Room) remove(connID string) {
	if c, ok := r.clients[connID]; ok {
		delete(c.ConvIDs, r.convID)
		delete(r.clients, connID)
	}
}

// broadcastRaw sends already-marshalled bytes to every member except one.
// It returns the clients whose send buffer was full (they must be dropped).
func (r *Room) broadcastRaw(b []byte, exceptConnID string) []*Client {
	var stale []*Client
	for id, c := range r.clients {
		if id == exceptConnID {
			continue
		}
		if !c.enqueue(b) {
			stale = append(stale, c)
		}
	}
	return stale
}

func (r *Room) empty() bool { return len(r.clients) == 0 }
