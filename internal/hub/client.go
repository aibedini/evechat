package hub

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/rs/zerolog"
)

// ClientKind distinguishes the two socket audiences.
type ClientKind int

const (
	// KindVisitor is a widget socket.
	KindVisitor ClientKind = iota
	// KindAgent is a dashboard socket.
	KindAgent
)

// Client is one websocket connection. Multiple connections per identity are
// allowed (multi-tab, phone + desktop).
type Client struct {
	ID        string // visitorID or agentID
	Kind      ClientKind
	ProjectID string
	ConnID    string // ULID, unique per socket
	Role      string // agent role (empty for visitors)
	Name      string // display name

	ConvIDs map[string]struct{}

	send      chan []byte
	conn      *websocket.Conn
	hub       *Hub
	log       zerolog.Logger
	closeOnce sync.Once
	done      chan struct{}
	lastSeen  atomic.Int64
	closed    atomic.Bool
	connected atomic.Bool
}

func newClient(h *Hub, conn *websocket.Conn, kind ClientKind, id, projectID, connID string) *Client {
	c := &Client{
		ID:        id,
		Kind:      kind,
		ProjectID: projectID,
		ConnID:    connID,
		ConvIDs:   make(map[string]struct{}, 4),
		send:      make(chan []byte, h.cfg.SendBufferSize),
		conn:      conn,
		hub:       h,
		done:      make(chan struct{}),
		log:       h.log.With().Str("conn_id", connID).Str("kind", kind.String()).Str("identity", id).Logger(),
	}
	c.lastSeen.Store(nowMS())
	return c
}

// KindName is used for logs and metrics.
func (k ClientKind) String() string {
	if k == KindAgent {
		return "agent"
	}
	return "visitor"
}

// IsAgent reports whether the client is a dashboard socket.
func (c *Client) IsAgent() bool { return c.Kind == KindAgent }

// Run starts the read and write loops and blocks until the socket ends.
// The caller must have registered the client already.
func (c *Client) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		c.writeLoop(ctx)
		cancel()
	}()
	c.readLoop(ctx)
	c.Close(websocket.StatusNormalClosure, "")
	<-c.done
}

func (c *Client) readLoop(ctx context.Context) {
	for {
		typ, data, err := c.conn.Read(ctx)
		if err != nil {
			return
		}
		c.lastSeen.Store(nowMS())
		if typ != websocket.MessageText {
			continue
		}
		if int64(len(data)) > c.hub.cfg.MaxMessageBytes {
			c.hub.log.Warn().Str("conn_id", c.ConnID).Int("bytes", len(data)).Msg("oversized frame dropped")
			continue
		}
		var e Envelope
		if err := json.Unmarshal(data, &e); err != nil {
			c.Send(ErrorEnvelope("invalid_request", "قالب پیام نامعتبر است"))
			continue
		}
		switch e.Type {
		case TypePing:
			c.Send(Envelope{Type: TypePong, TS: nowMS()})
			continue
		case TypePong:
			continue
		case "":
			continue
		}
		c.hub.dispatch(c, e)
	}
}

func (c *Client) writeLoop(ctx context.Context) {
	ticker := time.NewTicker(c.hub.cfg.PingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case b := <-c.send:
			if err := c.write(ctx, b); err != nil {
				return
			}
		case <-ticker.C:
			if elapsed := time.Since(time.UnixMilli(c.lastSeen.Load())); elapsed > c.hub.cfg.PongWait {
				c.log.Info().Dur("idle", elapsed).Msg("pong timeout, closing")
				c.Close(websocket.StatusPolicyViolation, "pong timeout")
				return
			}
			ping, err := json.Marshal(Envelope{Type: TypePing, TS: nowMS()})
			if err != nil {
				return
			}
			if err := c.write(ctx, ping); err != nil {
				return
			}
		}
	}
}

func (c *Client) write(ctx context.Context, b []byte) error {
	wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := c.conn.Write(wctx, websocket.MessageText, b); err != nil {
		if !errors.Is(err, context.Canceled) {
			c.log.Debug().Err(err).Msg("write failed")
		}
		c.Close(websocket.StatusInternalError, "write failed")
		return err
	}
	return nil
}

// enqueue is the non-blocking send used by the hub. It returns false when the
// buffer is full, which means the client must be dropped.
func (c *Client) enqueue(b []byte) bool {
	if c.closed.Load() {
		return false
	}
	select {
	case c.send <- b:
		return true
	default:
		c.hub.dropped.Add(1)
		c.log.Warn().Int("buffer", cap(c.send)).Msg("slow client dropped")
		return false
	}
}

// Send marshals and enqueues an envelope. It never blocks the caller.
func (c *Client) Send(e Envelope) {
	if e.TS == 0 {
		e.TS = nowMS()
	}
	b, err := json.Marshal(e)
	if err != nil {
		c.log.Error().Err(err).Str("type", e.Type).Msg("marshal envelope")
		return
	}
	if !c.enqueue(b) {
		c.Close(websocket.StatusPolicyViolation, "slow consumer")
	}
}

// SendRaw enqueues pre-marshalled bytes (used for room fan-out).
func (c *Client) SendRaw(b []byte) {
	if !c.enqueue(b) {
		c.Close(websocket.StatusPolicyViolation, "slow consumer")
	}
}

// Close shuts the socket down exactly once.
func (c *Client) Close(code websocket.StatusCode, reason string) {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		close(c.done)
		if c.conn != nil {
			_ = c.conn.Close(code, reason)
		}
	})
}

// Closed reports whether the socket is closed.
func (c *Client) Closed() bool { return c.closed.Load() }

// Touch marks activity on the socket.
func (c *Client) Touch() { c.lastSeen.Store(nowMS()) }
