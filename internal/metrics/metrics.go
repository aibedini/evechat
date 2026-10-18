// Package metrics is a small hand-written Prometheus exposition layer. It has
// no external dependency: counters, gauges and one histogram flavour are enough
// for EveChat's needs.
package metrics

import (
	"fmt"
	"io"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Buckets are the latency buckets used by every histogram (seconds).
var Buckets = []float64{0.005, 0.01, 0.05, 0.1, 0.5, 1, 5}

// BuildInfo identifies the running binary.
type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
	GoVersion string
}

type series struct {
	labels map[string]string
	value  float64
}

type histogram struct {
	labels  map[string]string
	buckets []float64
	counts  []uint64
	sum     float64
	count   uint64
}

type metricKind int

const (
	kindCounter metricKind = iota
	kindGauge
	kindHistogram
)

// Registry holds every metric and renders the Prometheus text format.
type Registry struct {
	mu          sync.RWMutex
	kinds       map[string]metricKind
	help        map[string]string
	counters    map[string]map[string]*series
	gauges      map[string]map[string]float64
	gaugeLabels map[string]map[string]map[string]string
	gaugesFn    []func()
	hists       map[string]map[string]*histogram
	started     time.Time
	build       BuildInfo
}

// New builds an empty registry.
func New(build BuildInfo) *Registry {
	if build.GoVersion == "" {
		build.GoVersion = runtime.Version()
	}
	r := &Registry{
		kinds:       map[string]metricKind{},
		help:        map[string]string{},
		counters:    map[string]map[string]*series{},
		gauges:      map[string]map[string]float64{},
		gaugeLabels: map[string]map[string]map[string]string{},
		hists:       map[string]map[string]*histogram{},
		started:     time.Now(),
		build:       build,
	}
	r.registerRuntime()
	return r
}

func (r *Registry) registerRuntime() {
	r.GaugeFunc("evechat_uptime_seconds", nil, func() float64 { return time.Since(r.started).Seconds() })
	r.GaugeFunc("evechat_goroutines", nil, func() float64 { return float64(runtime.NumGoroutine()) })
	r.GaugeFunc("evechat_memory_bytes", map[string]string{"kind": "heap"}, func() float64 {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return float64(m.HeapAlloc)
	})
	r.GaugeFunc("evechat_memory_bytes", map[string]string{"kind": "sys"}, func() float64 {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return float64(m.Sys)
	})
}

// Build returns the build info baked into the binary.
func (r *Registry) Build() BuildInfo { return r.build }

func labelsKey(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
	}
	return b.String()
}

// Counter increments a counter series.
func (r *Registry) Counter(name string, labels map[string]string, delta float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kinds[name] = kindCounter
	if r.counters[name] == nil {
		r.counters[name] = map[string]*series{}
	}
	key := labelsKey(labels)
	s, ok := r.counters[name][key]
	if !ok {
		s = &series{labels: copyLabels(labels)}
		r.counters[name][key] = s
	}
	s.value += delta
}

// Inc is Counter(name, labels, 1).
func (r *Registry) Inc(name string, labels map[string]string) { r.Counter(name, labels, 1) }

// SetGauge sets a gauge series from a callback-free value.
func (r *Registry) SetGauge(name string, labels map[string]string, v float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kinds[name] = kindGauge
	if r.gauges[name] == nil {
		r.gauges[name] = map[string]float64{}
	}
	r.gauges[name][labelsKey(labels)] = v
	if r.gaugeLabels == nil {
		r.gaugeLabels = map[string]map[string]map[string]string{}
	}
	if r.gaugeLabels[name] == nil {
		r.gaugeLabels[name] = map[string]map[string]string{}
	}
	r.gaugeLabels[name][labelsKey(labels)] = copyLabels(labels)
}

// GaugeFunc registers a gauge evaluated at scrape time.
func (r *Registry) GaugeFunc(name string, labels map[string]string, fn func() float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kinds[name] = kindGauge
	r.gaugesFn = append(r.gaugesFn, func() {
		r.SetGauge(name, labels, fn())
	})
}

// Observe records a histogram sample.
func (r *Registry) Observe(name string, labels map[string]string, v float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kinds[name] = kindHistogram
	if r.hists[name] == nil {
		r.hists[name] = map[string]*histogram{}
	}
	key := labelsKey(labels)
	h, ok := r.hists[name][key]
	if !ok {
		h = &histogram{labels: copyLabels(labels), buckets: Buckets, counts: make([]uint64, len(Buckets))}
		r.hists[name][key] = h
	}
	for i, b := range h.buckets {
		if v <= b {
			h.counts[i]++
		}
	}
	h.sum += v
	h.count++
}

func copyLabels(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// Render writes the Prometheus text exposition.
func (r *Registry) Render(w io.Writer) {
	// Gauge callbacks call SetGauge, which locks the registry: copy the list and
	// evaluate it BEFORE taking the lock, otherwise Render self-deadlocks.
	r.mu.Lock()
	callbacks := append([]func(){}, r.gaugesFn...)
	r.mu.Unlock()
	for _, fn := range callbacks {
		fn()
	}
	r.mu.Lock()
	snapshot := r.snapshotLocked()
	r.mu.Unlock()

	_, _ = fmt.Fprintf(w, "evechat_build_info{version=%q,commit=%q,go_version=%q,build_date=%q} 1\n",
		snapshot.build.Version, snapshot.build.Commit, snapshot.build.GoVersion, snapshot.build.BuildDate)

	for _, name := range snapshot.names {
		kind := snapshot.kinds[name]
		switch kind {
		case kindCounter:
			_, _ = fmt.Fprintf(w, "# TYPE %s counter\n", name)
			for _, key := range sortedKeys(snapshot.counters[name]) {
				s := snapshot.counters[name][key]
				_, _ = fmt.Fprintf(w, "%s%s %s\n", name, formatLabels(s.labels), formatFloat(s.value))
			}
		case kindGauge:
			_, _ = fmt.Fprintf(w, "# TYPE %s gauge\n", name)
			for _, key := range snapshot.gaugeKeys[name] {
				_, _ = fmt.Fprintf(w, "%s%s %s\n", name, formatLabels(snapshot.gaugeLabels[name][key]), formatFloat(snapshot.gauges[name][key]))
			}
		case kindHistogram:
			_, _ = fmt.Fprintf(w, "# TYPE %s histogram\n", name)
			for _, key := range sortedKeys(snapshot.hists[name]) {
				h := snapshot.hists[name][key]
				var cum uint64
				for i, b := range h.buckets {
					cum = h.counts[i]
					_, _ = fmt.Fprintf(w, "%s_bucket%s %d\n", name, formatLabels(withLE(h.labels, b)), cum)
				}
				_, _ = fmt.Fprintf(w, "%s_bucket%s %d\n", name, formatLabels(withLE(h.labels, 1e9)), h.count)
				_, _ = fmt.Fprintf(w, "%s_sum%s %s\n", name, formatLabels(h.labels), formatFloat(h.sum))
				_, _ = fmt.Fprintf(w, "%s_count%s %d\n", name, formatLabels(h.labels), h.count)
			}
		}
	}
}

type snapshotData struct {
	build       BuildInfo
	kinds       map[string]metricKind
	names       []string
	counters    map[string]map[string]*series
	gauges      map[string]map[string]float64
	gaugeLabels map[string]map[string]map[string]string
	gaugeKeys   map[string][]string
	hists       map[string]map[string]*histogram
}

func (r *Registry) snapshotLocked() snapshotData {
	s := snapshotData{
		build:       r.build,
		kinds:       map[string]metricKind{},
		counters:    map[string]map[string]*series{},
		gauges:      map[string]map[string]float64{},
		gaugeLabels: map[string]map[string]map[string]string{},
		gaugeKeys:   map[string][]string{},
		hists:       map[string]map[string]*histogram{},
	}
	for name, kind := range r.kinds {
		s.kinds[name] = kind
		s.names = append(s.names, name)
	}
	// Deep copy: the snapshot is rendered after the mutex is released, so it
	// must never alias a map that writers can still mutate (that caused a
	// "concurrent map iteration and map write" fatal under load).
	for name, m := range r.counters {
		cp := make(map[string]*series, len(m))
		for key, ser := range m {
			c := *ser
			c.labels = copyLabels(ser.labels)
			cp[key] = &c
		}
		s.counters[name] = cp
	}
	for name, m := range r.gauges {
		cp := make(map[string]float64, len(m))
		for key, v := range m {
			cp[key] = v
			s.gaugeKeys[name] = append(s.gaugeKeys[name], key)
		}
		sort.Strings(s.gaugeKeys[name])
		s.gauges[name] = cp
	}
	for name, m := range r.gaugeLabels {
		cp := make(map[string]map[string]string, len(m))
		for key, labels := range m {
			cp[key] = copyLabels(labels)
		}
		s.gaugeLabels[name] = cp
	}
	for name, m := range r.hists {
		cp := make(map[string]*histogram, len(m))
		for key, h := range m {
			c := *h
			c.counts = append([]uint64(nil), h.counts...)
			c.buckets = append([]float64(nil), h.buckets...)
			c.labels = copyLabels(h.labels)
			cp[key] = &c
		}
		s.hists[name] = cp
	}
	sort.Strings(s.names)
	return s
}

func withLE(labels map[string]string, le float64) map[string]string {
	out := copyLabels(labels)
	if out == nil {
		out = map[string]string{}
	}
	out["le"] = strconv.FormatFloat(le, 'g', -1, 64)
	return out
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", k, labels[k]))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func sortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---- process-wide registry used by instrumentation points ----

var (
	defaultMu sync.RWMutex
	defaultR  *Registry
)

// SetDefault installs the process registry.
func SetDefault(r *Registry) {
	defaultMu.Lock()
	defaultR = r
	defaultMu.Unlock()
}

// Get returns the process registry, creating a no-op one if needed.
func Get() *Registry {
	defaultMu.RLock()
	r := defaultR
	defaultMu.RUnlock()
	if r != nil {
		return r
	}
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultR == nil {
		defaultR = New(BuildInfo{Version: "dev"})
	}
	return defaultR
}
