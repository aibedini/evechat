package metrics

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

// TestRenderConcurrentWithUpdates guards the class of bug that shipped once:
// Render evaluated gauge callbacks while holding the registry mutex, and those
// callbacks call SetGauge, which takes the same mutex — a self-deadlock that a
// normal unit test never notices.
func TestRenderConcurrentWithUpdates(t *testing.T) {
	r := New(BuildInfo{Version: "test"})
	r.GaugeFunc("test_live", nil, func() float64 {
		// Deliberately mutate through the public API, like the runtime gauges do.
		r.SetGauge("test_live_inner", nil, float64(time.Now().UnixNano()%1000))
		return 42
	})

	stop := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					r.Counter("test_events", map[string]string{"kind": "write"}, 1)
					r.SetGauge("test_gauge", nil, float64(n))
					r.Observe("test_latency", map[string]string{"route": "/api"}, 0.01)
				}
			}
		}(i)
	}

	var scrapes sync.WaitGroup
	for i := 0; i < 32; i++ {
		scrapes.Add(1)
		go func() {
			defer scrapes.Done()
			for j := 0; j < 20; j++ {
				var buf bytes.Buffer
				r.Render(&buf)
				if buf.Len() == 0 {
					t.Error("empty render output")
					return
				}
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		scrapes.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		close(stop)
		wg.Wait()
		t.Fatal("Render deadlocked under concurrent updates")
	}
	close(stop)
	wg.Wait()
}

// TestRenderOutputIsStable verifies two consecutive scrapes agree on the metric
// families while values keep changing.
func TestRenderOutputIsStable(t *testing.T) {
	r := New(BuildInfo{Version: "test"})
	r.Inc("requests", nil)
	var a, b bytes.Buffer
	r.Render(&a)
	r.Inc("requests", nil)
	r.Render(&b)
	if !bytes.Contains(a.Bytes(), []byte("requests")) || !bytes.Contains(b.Bytes(), []byte("requests")) {
		t.Fatalf("requests family missing: %q / %q", a.String(), b.String())
	}
}
