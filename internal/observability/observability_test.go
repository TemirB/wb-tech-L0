package observability

import (
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// header.go file tests
func TestAppendServerTiming(t *testing.T) {
	tests := []struct {
		testName string

		name  string
		durMs float64
		desc  string

		expected string
	}{
		{
			testName: "durMs - ok, desc - ok",

			name:  "test",
			durMs: 100.5,
			desc:  "description",

			expected: `test;dur=100.50;desc="description"`,
		},
		{
			testName: "durMs - ok, desc is empty",

			name:  "test",
			durMs: 200.0,
			desc:  "",

			expected: "test;dur=200.00",
		},
		{
			testName: "durMs is zero, desc is ok",

			name:  "test",
			durMs: 0,
			desc:  "description",

			expected: `test;desc="description"`,
		},
		{
			testName: "durMs is zero, desc is empty",

			name:  "test",
			durMs: 0,
			desc:  "",

			expected: "",
		},
		{
			testName: "durMs is negative, desc is ok",

			name:  "test",
			durMs: -10,
			desc:  "description",

			expected: `test;desc="description"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			AppendServerTiming(w, tt.name, tt.durMs, tt.desc)

			result := w.Header().Get("Server-Timing")
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestAppendServerTiming_MultipleCalls(t *testing.T) {
	w := httptest.NewRecorder()

	AppendServerTiming(w, "db", 150.25, "database query")
	expected1 := `db;dur=150.25;desc="database query"`
	result1 := w.Header().Get("Server-Timing")
	require.Equal(t, expected1, result1)

	AppendServerTiming(w, "cache", 50.0, "cache lookup")

	headers := w.Header()["Server-Timing"]
	if len(headers) != 2 {
		t.Errorf("Expected 2 Server-Timing headers, got %d", len(headers))
	}

	expected2 := `cache;dur=50.00;desc="cache lookup"`
	require.Equal(t, expected1, headers[0])
	require.Equal(t, expected2, headers[1])
}

func TestSetIfPos(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		ms       float64
		expected string
	}{
		{
			name: "ms is positive",

			key:      "X-Response-Time",
			ms:       123.45,
			expected: "123.45",
		},
		{
			name: "ms is zero",

			key:      "X-Response-Time",
			ms:       0,
			expected: "",
		},
		{
			name: "ms is negative",

			key:      "X-Response-Time",
			ms:       -10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			SetIfPos(w, tt.key, tt.ms)

			result := w.Header().Get(tt.key)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestSetIfPos_Overwrite(t *testing.T) {
	w := httptest.NewRecorder()

	SetIfPos(w, "X-Time", 100.0)
	expected1 := "100.00"
	result1 := w.Header().Get("X-Time")
	require.Equal(t, expected1, result1)

	SetIfPos(w, "X-Time", 200.0)
	expected2 := "200.00"
	result2 := w.Header().Get("X-Time")
	require.Equal(t, expected2, result2)

	SetIfPos(w, "X-Time", -50.0)
	result3 := w.Header().Get("X-Time")
	require.Equal(t, expected2, result3)
}

func TestSetIfPos_ZeroDoesNotSetHeader(t *testing.T) {
	w := httptest.NewRecorder()

	SetIfPos(w, "X-Time", 100.0)

	SetIfPos(w, "X-Time", 0)

	result := w.Header().Get("X-Time")

	require.Equal(t, "100.00", result)
}

// inmem.go file tests
func TestInmem_push(t *testing.T) {
	tests := []struct {
		name     string
		max      int
		pushes   []*observe
		expected []*observe
	}{
		{
			name:     "basic push within limits",
			max:      3,
			pushes:   []*observe{{Kind: "a"}, {Kind: "b"}, {Kind: "c"}},
			expected: []*observe{{Kind: "a"}, {Kind: "b"}, {Kind: "c"}},
		},
		{
			name:     "push beyond max size",
			max:      2,
			pushes:   []*observe{{Kind: "a"}, {Kind: "b"}, {Kind: "c"}},
			expected: []*observe{{Kind: "b"}, {Kind: "c"}},
		},
		{
			name:     "multiple overflows",
			max:      2,
			pushes:   []*observe{{Kind: "a"}, {Kind: "b"}, {Kind: "c"}, {Kind: "d"}, {Kind: "e"}},
			expected: []*observe{{Kind: "d"}, {Kind: "e"}},
		},
		{
			name:     "zero max size",
			max:      0,
			pushes:   []*observe{{Kind: "a"}, {Kind: "b"}, {Kind: "c"}},
			expected: []*observe{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inmem := &Inmem{max: tt.max}
			for _, item := range tt.pushes {
				inmem.push(item)
			}

			require.Equal(t, tt.expected, inmem.last)
			// for i := range tt.expected {
			// 	require.Equal(t, tt.expected[i], inmem.last[i])
			// }
		})
	}
}

func TestInmem_ObserveMethods(t *testing.T) {
	tests := []struct {
		name     string
		action   func(m *Inmem)
		expected struct {
			length int
			kind   string
		}
	}{
		{
			name: "ObserveLookup",
			action: func(m *Inmem) {
				m.ObserveLookup("test-source", 10.5, 25.3)
			},
			expected: struct {
				length int
				kind   string
			}{length: 1, kind: "lookup"},
		},
		{
			name: "ObserveUpsert",
			action: func(m *Inmem) {
				m.ObserveUpsert(15.7)
			},
			expected: struct {
				length int
				kind   string
			}{length: 1, kind: "upsert"},
		},
		{
			name: "ObserveHTTP",
			action: func(m *Inmem) {
				m.ObserveHTTP("GET", "/api/test", 200, 45.2)
			},
			expected: struct {
				length int
				kind   string
			}{length: 1, kind: "http"},
		},
		{
			name: "ObserveKafka",
			action: func(m *Inmem) {
				m.ObserveKafka(30.1, true)
			},
			expected: struct {
				length int
				kind   string
			}{length: 1, kind: "kafka"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inmem := &Inmem{max: 10}
			tt.action(inmem)

			require.Equal(t, tt.expected.length, len(inmem.last))

			entry := inmem.last[0]
			require.Equal(t, entry.Kind, tt.expected.kind)
		})
	}
}

func TestInmem_IncCacheCounters(t *testing.T) {
	tests := []struct {
		name           string
		actions        func(m *Inmem)
		expectedHits   int
		expectedMisses int
	}{
		{
			name: "single hit",
			actions: func(m *Inmem) {
				m.IncCacheHit()
			},
			expectedHits:   1,
			expectedMisses: 0,
		},
		{
			name: "single miss",
			actions: func(m *Inmem) {
				m.IncCacheMiss()
			},
			expectedHits:   0,
			expectedMisses: 1,
		},
		{
			name: "multiple hits",
			actions: func(m *Inmem) {
				m.IncCacheHit()
				m.IncCacheHit()
				m.IncCacheHit()
			},
			expectedHits:   3,
			expectedMisses: 0,
		},
		{
			name: "multiple misses",
			actions: func(m *Inmem) {
				m.IncCacheMiss()
				m.IncCacheMiss()
			},
			expectedHits:   0,
			expectedMisses: 2,
		},
		{
			name: "mixed hits and misses",
			actions: func(m *Inmem) {
				m.IncCacheHit()
				m.IncCacheMiss()
				m.IncCacheHit()
				m.IncCacheMiss()
				m.IncCacheHit()
			},
			expectedHits:   3,
			expectedMisses: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inmem := NewInmem(10)
			tt.actions(inmem)

			if inmem.totals.cacheHits != tt.expectedHits {
				t.Errorf("Expected %d cache hits, got %d", tt.expectedHits, inmem.totals.cacheHits)
			}
			if inmem.totals.cacheMiss != tt.expectedMisses {
				t.Errorf("Expected %d cache misses, got %d", tt.expectedMisses, inmem.totals.cacheMiss)
			}
		})
	}
}

func TestInmem_ConcurrentOperations(t *testing.T) {
	inmem := &Inmem{max: 100}
	var wg sync.WaitGroup

	// Concurrent pushes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			val := &observe{
				Kind: strconv.Itoa(i),
			}
			inmem.push(val)
		}(i)
	}

	// Concurrent increments
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			inmem.IncCacheHit()
		}()
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			inmem.IncCacheMiss()
		}()
	}

	wg.Wait()

	require.Equal(t, 50, len(inmem.last))
	require.Equal(t, 30, inmem.totals.cacheHits)
	require.Equal(t, 20, inmem.totals.cacheMiss)
}

func TestInmem_NegativeValues(t *testing.T) {
	inmem := &Inmem{max: 10}

	// Methods should handle negative values without panicking
	inmem.ObserveLookup("test", -1.0, -2.0)
	inmem.ObserveUpsert(-5.0)
	inmem.ObserveHTTP("GET", "/", 200, -10.0)
	inmem.ObserveKafka(-3.0, false)

	if len(inmem.last) != 4 {
		t.Errorf("Expected 4 observations, got %d", len(inmem.last))
	}
}
