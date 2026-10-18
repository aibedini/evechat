package hub

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/evechat/evechat/internal/id"
	"github.com/rs/zerolog"
)

// HubConfig is the runtime tuning of the hub.
type HubConfig struct {
	PingInterval    time.Duration
	PongWait        time.Duration
	SendBufferSize  int
	RingBufferSize  int
	MaxMessageBytes int64
}

// HubStats is a snapshot of hub state.
type HubStats struct {
	Connections           int    `json:"connections"`
	Visitors              int    `json:"visitors"`
	Agents                int    `json:"agents"`
	Rooms                 int    `json:"rooms"`
	BufferedConversations int    `json:"buffered_conversations"`
	Dropped               uint64 `json:"dropped"`
	Delivered             uint64 `json:"delivered"`
}

// MessageHandler is implemented by the services layer and injected at startup.
// Every method runs on its own goroutine, never on the hub goroutine.
type MessageHandler interface {
	OnVisitorMessage(ctx context.Context, c *Client, e Envelope) error
	OnAgentMessage(ctx context.Context, c *Client, e Envelope) error
	OnVisitorConnect(ctx context.Context, c *Client) error
	OnVisitorDisconnect(ctx context.Context, c *Client)
	OnAgentConnect(ctx context.Context, c *Client) error
	OnAgentDisconnect(ctx context.Context, c *Client)
}

type command func()

// Hub owns all connection state. Exactly one goroutine (Run) mutates it; every
// exported method only pushes a command onto cmds.
type Hub struct {
	cmds chan command
	done chan struct{}

	stopOnce sync.Once

	visitors      map[string]map[string]*Client
	agents        map[string]map[string]*Client
	agentProject  map[string]string
	projectAgents map[string]map[string]struct{}
	rooms         map[string]*Room

	bufs *MsgBuffers

	handler MessageHandler
	cfg     HubConfig
	log     zerolog.Logger

	dropped   atomic.Uint64
	delivered atomic.Uint64
	sem       chan struct{}
}

// New builds a hub. Start it with Run.
func New(cfg HubConfig, log zerolog.Logger) *Hub {
	if cfg.PingInterval <= 0 {
		cfg.PingInterval = 30 * time.Second
	}
	if cfg.PongWait <= 0 {
		cfg.PongWait = 90 * time.Second
	}
	if cfg.SendBufferSize <= 0 {
		cfg.SendBufferSize = 256
	}
	if cfg.RingBufferSize <= 0 {
		cfg.RingBufferSize = 50
	}
	if cfg.MaxMessageBytes <= 0 {
		cfg.MaxMessageBytes = 32768
	}
	return &Hub{
		cmds:          make(chan command, 4096),
		done:          make(chan struct{}),
		visitors:      make(map[string]map[string]*Client, 128),
		agents:        make(map[string]map[string]*Client, 32),
		agentProject:  make(map[string]string, 32),
		projectAgents: make(map[string]map[string]struct{}, 8),
		rooms:         make(map[string]*Room, 256),
		bufs:          NewMsgBuffers(cfg.RingBufferSize),
		cfg:           cfg,
		log:           log,
		sem:           make(chan struct{}, 512),
	}
}

// SetHandler injects the services handler. Call before Run.
func (h *Hub) SetHandler(m MessageHandler) { h.handler = m }

// Run is the single hub goroutine.
func (h *Hub) Run(ctx context.Context) {
	statsTick := time.NewTicker(30 * time.Second)
	gcTick := time.NewTicker(5 * time.Minute)
	defer statsTick.Stop()
	defer gcTick.Stop()
	defer h.stop()

	for {
		select {
		case <-ctx.Done():
			h.closeAll(websocket.StatusGoingAway, "server shutting down")
			return
		case <-h.done:
			return
		case fn := <-h.cmds:
			fn()
		case <-statsTick.C:
			h.broadcastStats()
		case <-gcTick.C:
			h.gc()
		}
	}
}

func (h *Hub) stop() {
	h.stopOnce.Do(func() { close(h.done) })
}

// do enqueues fn and never blocks past shutdown.
func (h *Hub) do(fn command) {
	select {
	case h.cmds <- fn:
	case <-h.done:
	}
}

// NewClient wraps an accepted websocket connection.
func (h *Hub) NewClient(conn *websocket.Conn, kind ClientKind, identity, projectID, role, name string) *Client {
	c := newClient(h, conn, kind, identity, projectID, id.New())
	c.Role = role
	c.Name = name
	return c
}

// Register adds a client to the hub and notifies the handler.
func (h *Hub) Register(c *Client) {
	h.do(func() {
		h.addLocked(c)
		c.connected.Store(true)
		h.log.Info().Str("conn_id", c.ConnID).Str("kind", c.Kind.String()).Str("identity", c.ID).Msg("client registered")
	})
	h.notifyConnect(c)
}

func (h *Hub) addLocked(c *Client) {
	if c.Kind == KindAgent {
		if h.agents[c.ID] == nil {
			h.agents[c.ID] = make(map[string]*Client, 2)
		}
		h.agents[c.ID][c.ConnID] = c
		h.agentProject[c.ID] = c.ProjectID
		if h.projectAgents[c.ProjectID] == nil {
			h.projectAgents[c.ProjectID] = make(map[string]struct{}, 4)
		}
		h.projectAgents[c.ProjectID][c.ID] = struct{}{}
		return
	}
	if h.visitors[c.ID] == nil {
		h.visitors[c.ID] = make(map[string]*Client, 2)
	}
	h.visitors[c.ID][c.ConnID] = c
}

// Unregister removes a client and notifies the handler.
func (h *Hub) Unregister(c *Client) {
	if c == nil {
		return
	}
	wasConnected := c.connected.Swap(false)
	h.do(func() { h.removeLocked(c) })
	if wasConnected {
		h.notifyDisconnect(c)
	} else {
		// Still ensure removal in case Register's command is processed later.
		h.do(func() { h.removeLocked(c) })
	}
}

func (h *Hub) removeLocked(c *Client) {
	if c.Kind == KindAgent {
		if conns, ok := h.agents[c.ID]; ok {
			delete(conns, c.ConnID)
			if len(conns) == 0 {
				delete(h.agents, c.ID)
				delete(h.agentProject, c.ID)
				if pa, ok := h.projectAgents[c.ProjectID]; ok {
					delete(pa, c.ID)
					if len(pa) == 0 {
						delete(h.projectAgents, c.ProjectID)
					}
				}
			}
		}
	} else if conns, ok := h.visitors[c.ID]; ok {
		delete(conns, c.ConnID)
		if len(conns) == 0 {
			delete(h.visitors, c.ID)
		}
	}
	for convID := range c.ConvIDs {
		if r, ok := h.rooms[convID]; ok {
			r.remove(c.ConnID)
			if r.empty() {
				delete(h.rooms, convID)
			}
		}
	}
	c.Close(websocket.StatusNormalClosure, "")
}

func (h *Hub) notifyConnect(c *Client) {
	if h.handler == nil {
		return
	}
	handler := h.handler
	go func() {
		defer h.recoverPanic("connect")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var err error
		if c.Kind == KindAgent {
			err = handler.OnAgentConnect(ctx, c)
		} else {
			err = handler.OnVisitorConnect(ctx, c)
		}
		if err != nil {
			h.log.Error().Err(err).Str("conn_id", c.ConnID).Msg("connect handler failed")
			c.Send(ErrorEnvelope("internal", "خطا در آماده‌سازی اتصال"))
		}
	}()
}

func (h *Hub) notifyDisconnect(c *Client) {
	if h.handler == nil {
		return
	}
	handler := h.handler
	go func() {
		defer h.recoverPanic("disconnect")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if c.Kind == KindAgent {
			handler.OnAgentDisconnect(ctx, c)
		} else {
			handler.OnVisitorDisconnect(ctx, c)
		}
	}()
}

func (h *Hub) dispatch(c *Client, e Envelope) {
	if h.handler == nil {
		return
	}
	kind := c.Kind
	select {
	case h.sem <- struct{}{}:
	default:
		c.Send(ErrorEnvelope("rate_limited", "سرور در حال حاضر مشغول است"))
		return
	}
	// Spawned from the hub goroutine so hub state stays single-threaded while
	// database work never blocks it.
	go func() {
		defer func() { <-h.sem }()
		defer h.recoverPanic("message")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var err error
		if kind == KindAgent {
			err = h.handler.OnAgentMessage(ctx, c, e)
		} else {
			err = h.handler.OnVisitorMessage(ctx, c, e)
		}
		if err != nil {
			h.log.Error().Err(err).Str("type", e.Type).Str("identity", c.ID).Msg("message handler failed")
			c.Send(ErrorEnvelope("internal", "پردازش پیام ناموفق بود"))
		}
	}()
}

func (h *Hub) recoverPanic(where string) {
	if r := recover(); r != nil {
		h.log.Error().Interface("panic", r).Str("where", where).Msg("recovered panic in hub handler")
	}
}

// JoinRoom subscribes a client to a conversation.
func (h *Hub) JoinRoom(convID string, c *Client) {
	if convID == "" || c == nil {
		return
	}
	h.do(func() {
		r, ok := h.rooms[convID]
		if !ok {
			r = newRoom(convID)
			h.rooms[convID] = r
		}
		r.add(c)
	})
}

// LeaveRoom unsubscribes a client from a conversation.
func (h *Hub) LeaveRoom(convID string, c *Client) {
	if convID == "" || c == nil {
		return
	}
	h.do(func() {
		r, ok := h.rooms[convID]
		if !ok {
			return
		}
		r.remove(c.ConnID)
		if r.empty() {
			delete(h.rooms, convID)
		}
	})
}

func marshalEnvelope(e Envelope) []byte {
	if e.TS == 0 {
		e.TS = nowMS()
	}
	b, err := json.Marshal(e)
	if err != nil {
		return nil
	}
	return b
}

// ToConversation broadcasts an ephemeral envelope to a room (not buffered).
func (h *Hub) ToConversation(convID string, e Envelope, exceptConnID string) {
	if convID == "" {
		return
	}
	b := marshalEnvelope(e)
	if b == nil {
		return
	}
	h.do(func() { h.broadcastLocked(convID, b, exceptConnID) })
}

// ToConversationAndBuffer records the envelope for sync replay and broadcasts it.
func (h *Hub) ToConversationAndBuffer(convID string, e Envelope, exceptConnID string) {
	if convID == "" {
		return
	}
	if e.TS == 0 {
		e.TS = nowMS()
	}
	h.bufs.Append(convID, e)
	b := marshalEnvelope(e)
	if b == nil {
		return
	}
	h.do(func() { h.broadcastLocked(convID, b, exceptConnID) })
}

func (h *Hub) broadcastLocked(convID string, b []byte, exceptConnID string) {
	r, ok := h.rooms[convID]
	if !ok {
		return
	}
	stale := r.broadcastRaw(b, exceptConnID)
	h.delivered.Add(uint64(len(r.clients) - len(stale)))
	for _, c := range stale {
		h.removeLocked(c)
		if h.handler != nil && c.connected.Swap(false) {
			h.notifyDisconnect(c)
		}
	}
	if r.empty() {
		delete(h.rooms, convID)
	}
}

// JoinRoomForAgents subscribes every socket of the given agents to a room.
// Used after assignment/transfer without needing the *Client handles.
func (h *Hub) JoinRoomForAgents(convID string, agentIDs ...string) {
	if convID == "" || len(agentIDs) == 0 {
		return
	}
	h.do(func() {
		r, ok := h.rooms[convID]
		if !ok {
			r = newRoom(convID)
			h.rooms[convID] = r
		}
		for _, agentID := range agentIDs {
			for _, c := range h.agents[agentID] {
				r.add(c)
			}
		}
	})
}

// JoinRoomForVisitor subscribes every socket of a visitor to a room.
func (h *Hub) JoinRoomForVisitor(convID, visitorID string) {
	if convID == "" || visitorID == "" {
		return
	}
	h.do(func() {
		r, ok := h.rooms[convID]
		if !ok {
			r = newRoom(convID)
			h.rooms[convID] = r
		}
		for _, c := range h.visitors[visitorID] {
			r.add(c)
		}
	})
}

// LeaveRoomForAgents unsubscribes the sockets of the given agents.
func (h *Hub) LeaveRoomForAgents(convID string, agentIDs ...string) {
	if convID == "" {
		return
	}
	h.do(func() {
		r, ok := h.rooms[convID]
		if !ok {
			return
		}
		for _, agentID := range agentIDs {
			for _, c := range h.agents[agentID] {
				r.remove(c.ConnID)
			}
		}
		if r.empty() {
			delete(h.rooms, convID)
		}
	})
}

// ToVisitor sends an envelope to every socket of one visitor.
func (h *Hub) ToVisitor(visitorID string, e Envelope) {
	b := marshalEnvelope(e)
	if b == nil {
		return
	}
	h.do(func() {
		conns := h.visitors[visitorID]
		for _, c := range conns {
			if !c.enqueue(b) {
				h.removeLocked(c)
			}
		}
		if len(conns) > 0 {
			h.delivered.Add(uint64(len(conns)))
		}
	})
}

// ToAgent sends an envelope to every socket of one agent.
func (h *Hub) ToAgent(agentID string, e Envelope) {
	b := marshalEnvelope(e)
	if b == nil {
		return
	}
	h.do(func() {
		conns := h.agents[agentID]
		for _, c := range conns {
			if !c.enqueue(b) {
				h.removeLocked(c)
			}
		}
		if len(conns) > 0 {
			h.delivered.Add(uint64(len(conns)))
		}
	})
}

// ToProjectAgents fans an envelope out to every agent socket of a project.
func (h *Hub) ToProjectAgents(projectID string, e Envelope) {
	b := marshalEnvelope(e)
	if b == nil {
		return
	}
	h.do(func() {
		agents := h.projectAgents[projectID]
		for agentID := range agents {
			for _, c := range h.agents[agentID] {
				if !c.enqueue(b) {
					h.removeLocked(c)
				}
			}
		}
		if len(agents) > 0 {
			h.delivered.Add(uint64(len(agents)))
		}
	})
}

// ToProjectAgentsExcept is ToProjectAgents without one agent identity.
func (h *Hub) ToProjectAgentsExcept(projectID, exceptAgentID string, e Envelope) {
	b := marshalEnvelope(e)
	if b == nil {
		return
	}
	h.do(func() {
		for agentID := range h.projectAgents[projectID] {
			if agentID == exceptAgentID {
				continue
			}
			for _, c := range h.agents[agentID] {
				if !c.enqueue(b) {
					h.removeLocked(c)
				}
			}
		}
	})
}

// OnlineAgents lists agent ids with at least one live socket in a project.
func (h *Hub) OnlineAgents(projectID string) []string {
	reply := make(chan []string, 1)
	h.do(func() {
		out := make([]string, 0, len(h.projectAgents[projectID]))
		for agentID := range h.projectAgents[projectID] {
			if len(h.agents[agentID]) > 0 {
				out = append(out, agentID)
			}
		}
		reply <- out
	})
	select {
	case v := <-reply:
		return v
	case <-h.done:
		return nil
	case <-time.After(5 * time.Second):
		return nil
	}
}

// IsVisitorOnline reports whether a visitor has a live socket.
func (h *Hub) IsVisitorOnline(visitorID string) bool {
	reply := make(chan bool, 1)
	h.do(func() { reply <- len(h.visitors[visitorID]) > 0 })
	select {
	case v := <-reply:
		return v
	case <-h.done:
		return false
	case <-time.After(5 * time.Second):
		return false
	}
}

// Stats returns a consistent snapshot of hub state.
func (h *Hub) Stats() HubStats {
	reply := make(chan HubStats, 1)
	h.do(func() {
		s := HubStats{
			Visitors:              len(h.visitors),
			Agents:                len(h.agents),
			Rooms:                 len(h.rooms),
			BufferedConversations: h.bufs.Count(),
			Dropped:               h.dropped.Load(),
			Delivered:             h.delivered.Load(),
		}
		for _, conns := range h.visitors {
			s.Connections += len(conns)
		}
		for _, conns := range h.agents {
			s.Connections += len(conns)
		}
		reply <- s
	})
	select {
	case s := <-reply:
		return s
	case <-h.done:
		return HubStats{}
	case <-time.After(5 * time.Second):
		return HubStats{}
	}
}

// SyncBuffer answers a sync.request from the in-memory ring.
func (h *Hub) SyncBuffer(convID string, sinceMS int64) ([]Envelope, bool) {
	return h.bufs.Since(convID, sinceMS)
}

// Record appends an outbound envelope to the conversation ring.
func (h *Hub) Record(convID string, e Envelope) { h.bufs.Append(convID, e) }

// DropSlow removes a client that failed to keep up.
func (h *Hub) DropSlow(c *Client) {
	if c == nil {
		return
	}
	h.do(func() { h.removeLocked(c) })
}

// Shutdown drains rooms and closes every socket with code 1001.
func (h *Hub) Shutdown(ctx context.Context) {
	done := make(chan struct{})
	h.do(func() {
		h.closeAll(websocket.StatusGoingAway, "server shutting down")
		close(done)
	})
	select {
	case <-done:
	case <-ctx.Done():
	case <-h.done:
	}
	h.stop()
}

func (h *Hub) closeAll(code websocket.StatusCode, reason string) {
	seen := make(map[*Client]struct{}, len(h.visitors)+len(h.agents))
	for _, conns := range h.visitors {
		for _, c := range conns {
			if _, ok := seen[c]; ok {
				continue
			}
			seen[c] = struct{}{}
			c.Close(code, reason)
		}
	}
	for _, conns := range h.agents {
		for _, c := range conns {
			if _, ok := seen[c]; ok {
				continue
			}
			seen[c] = struct{}{}
			c.Close(code, reason)
		}
	}
	h.visitors = make(map[string]map[string]*Client, 8)
	h.agents = make(map[string]map[string]*Client, 8)
	h.agentProject = make(map[string]string, 8)
	h.projectAgents = make(map[string]map[string]struct{}, 8)
	h.rooms = make(map[string]*Room, 8)
	h.log.Info().Int("connections", len(seen)).Msg("hub shut down")
}

func (h *Hub) broadcastStats() {
	if len(h.agents) == 0 {
		return
	}
	s := HubStats{
		Visitors: len(h.visitors),
		Agents:   len(h.agents),
		Rooms:    len(h.rooms),
		Dropped:  h.dropped.Load(),
	}
	for _, conns := range h.visitors {
		s.Connections += len(conns)
	}
	for _, conns := range h.agents {
		s.Connections += len(conns)
	}
	b := marshalEnvelope(NewEnvelope(TypeStatsTick, s))
	if b == nil {
		return
	}
	for _, conns := range h.agents {
		for _, c := range conns {
			if !c.enqueue(b) {
				h.removeLocked(c)
			}
		}
	}
}

func (h *Hub) gc() {
	dropped := h.bufs.GC()
	empty := 0
	for convID, r := range h.rooms {
		if r.empty() {
			delete(h.rooms, convID)
			empty++
		}
	}
	h.log.Debug().Int("buffers_evicted", dropped).Int("rooms_evicted", empty).Msg("hub gc")
}
