package hub

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func newTestHub(t *testing.T) (*Hub, context.CancelFunc) {
	t.Helper()
	h := New(HubConfig{
		PingInterval:    30 * time.Second,
		PongWait:        90 * time.Second,
		SendBufferSize:  64,
		RingBufferSize:  10,
		MaxMessageBytes: 32768,
	}, zerolog.Nop())
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	return h, cancel
}

func fakeClient(h *Hub, kind ClientKind, identity, projectID string) *Client {
	c := &Client{
		ID:        identity,
		Kind:      kind,
		ProjectID: projectID,
		ConnID:    identity + "-conn",
		ConvIDs:   make(map[string]struct{}, 2),
		send:      make(chan []byte, h.cfg.SendBufferSize),
		hub:       h,
		done:      make(chan struct{}),
		log:       zerolog.Nop(),
	}
	c.lastSeen.Store(nowMS())
	return c
}

func TestHubBroadcastUnderLoad(t *testing.T) {
	h, cancel := newTestHub(t)
	defer func() {
		cancel()
		ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		h.Shutdown(ctx)
	}()

	const clients = 500
	const broadcasts = 10000

	list := make([]*Client, 0, clients)
	for i := 0; i < clients; i++ {
		c := fakeClient(h, KindVisitor, fmt.Sprintf("vis_%d", i), "proj_1")
		list = append(list, c)
		h.Register(c)
		h.JoinRoom("conv_1", c)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for _, c := range list {
		wg.Add(1)
		go func(c *Client) {
			defer wg.Done()
			for {
				select {
				case <-c.send:
				case <-stop:
					return
				case <-c.done:
					return
				}
			}
		}(c)
	}

	done := make(chan struct{})
	go func() {
		for i := 0; i < broadcasts; i++ {
			h.ToConversation("conv_1", NewEnvelope(TypeMessageReceived, map[string]string{"i": fmt.Sprint(i)}), "")
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("broadcasts deadlocked")
	}

	// Stats goes through the same command channel: when it returns every
	// preceding command has been processed.
	stats := h.Stats()
	if stats.Connections != clients {
		t.Fatalf("expected %d connections, got %d", clients, stats.Connections)
	}
	if stats.Rooms != 1 {
		t.Fatalf("expected 1 room, got %d", stats.Rooms)
	}
	if stats.Delivered == 0 {
		t.Fatal("expected delivered counter to grow")
	}

	close(stop)
	wg.Wait()
}

func TestHubDropsStalledClient(t *testing.T) {
	h, cancel := newTestHub(t)
	defer func() {
		cancel()
		ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		h.Shutdown(ctx)
	}()

	fast := fakeClient(h, KindVisitor, "vis_fast", "proj_1")
	stalled := fakeClient(h, KindVisitor, "vis_stalled", "proj_1")
	stalled.send = make(chan []byte, 1)
	h.Register(fast)
	h.Register(stalled)
	h.JoinRoom("conv_9", fast)
	h.JoinRoom("conv_9", stalled)

	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for {
			select {
			case <-fast.send:
			case <-fast.done:
				return
			}
		}
	}()

	// First frame fills the stalled buffer; the second must get it dropped
	// instead of blocking the hub goroutine.
	for i := 0; i < 50; i++ {
		h.ToConversation("conv_9", NewEnvelope(TypeMessageReceived, map[string]int{"i": i}), "")
	}

	deadline := time.After(3 * time.Second)
	for {
		if h.Stats().Connections == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("stalled client was not dropped, connections=%d", h.Stats().Connections)
		case <-time.After(10 * time.Millisecond):
		}
	}
	fast.Close(0, "")
	<-drained
}

func TestHubNoGoroutineLeak(t *testing.T) {
	before := runtime.NumGoroutine()
	h, cancel := newTestHub(t)
	for i := 0; i < 100; i++ {
		c := fakeClient(h, KindAgent, fmt.Sprintf("agt_%d", i), "proj_1")
		h.Register(c)
		h.Unregister(c)
		c.Close(0, "")
	}
	h.Stats()
	cancel()
	ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	h.Shutdown(ctx)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= before+5 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("goroutines leaked: before=%d after=%d", before, runtime.NumGoroutine())
}

func TestMsgBuffersRing(t *testing.T) {
	b := NewMsgBuffers(3)
	for i := 1; i <= 5; i++ {
		b.Append("conv", Envelope{Type: TypeMessageReceived, TS: int64(i)})
	}
	if b.Len("conv") != 3 {
		t.Fatalf("expected ring size 3, got %d", b.Len("conv"))
	}
	got, ok := b.Since("conv", 3)
	if !ok {
		t.Fatal("expected buffer to answer for since=3")
	}
	if len(got) != 2 || got[0].TS != 4 {
		t.Fatalf("unexpected buffer content: %+v", got)
	}
	if _, ok := b.Since("conv", 0); ok {
		t.Fatal("expected buffer miss for trimmed history")
	}
	if _, ok := b.Since("missing", 0); ok {
		t.Fatal("expected buffer miss for unknown conversation")
	}
}
