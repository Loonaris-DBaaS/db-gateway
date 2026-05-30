package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSingleflight_ConcurrentCalls(t *testing.T) {
	hex64 := strings.Repeat("a", 64)
	parsed, _ := ParseTenantKey("sk_live_" + hex64 + "_rw")

	var apiCalls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls.Add(1)
		time.Sleep(100 * time.Millisecond)
		fixture := TenantRoute{
			TenantID:        "project-test1",
			PgBouncerRWHost: "pgbouncer-test1-rw",
			PgBouncerRWPort: 5432,
			PgBouncerROHost: "pgbouncer-test1-ro",
			PgBouncerROPort: 5432,
			Status:          "active",
		}
		json.NewEncoder(w).Encode(fixture)
	}))
	defer ts.Close()

	s := NewServer(Config{
		Address:         ":0",
		ControlPlaneURL: ts.URL,
		GatewaySecret:   "test-secret",
	})

	var wg sync.WaitGroup
	var errors []error
	var mu sync.Mutex

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.lookupRoute(parsed.KeyHash)
			if err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(errors) > 0 {
		t.Fatalf("unexpected errors: %v", errors)
	}

	calls := apiCalls.Load()
	if calls != 1 {
		t.Errorf("expected exactly 1 API call (singleflight), got %d", calls)
	}
}