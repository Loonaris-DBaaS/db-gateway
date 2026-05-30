package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLookupRoute_CacheMiss(t *testing.T) {
	hex64 := strings.Repeat("a", 64)
	parsed, _ := ParseTenantKey("sk_live_" + hex64 + "_rw")

	fixture := TenantRoute{
		TenantID:        "project-test1",
		PgBouncerRWHost: "pgbouncer-test1-rw",
		PgBouncerRWPort: 5432,
		PgBouncerROHost: "pgbouncer-test1-ro",
		PgBouncerROPort: 5432,
		Status:          "active",
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/routes/"+parsed.KeyHash {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(fixture)
	}))
	defer ts.Close()

	s := NewServer(Config{
		Address:         ":0",
		ControlPlaneURL: ts.URL,
		GatewaySecret:   "test-secret",
	})

	route, err := s.lookupRoute(parsed.KeyHash)
	if err != nil {
		t.Fatalf("lookupRoute: %v", err)
	}
	if route.TenantID != "project-test1" {
		t.Errorf("tenant_id=%s, want project-test1", route.TenantID)
	}
	if route.PgBouncerRWHost != "pgbouncer-test1-rw" {
		t.Errorf("rw_host=%s, want pgbouncer-test1-rw", route.PgBouncerRWHost)
	}
	if route.CachedAt.IsZero() {
		t.Error("CachedAt should be set on cache miss")
	}
}

func TestLookupRoute_CacheHit(t *testing.T) {
	hex64 := strings.Repeat("a", 64)
	parsed, _ := ParseTenantKey("sk_live_" + hex64 + "_rw")

	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
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

	_, err := s.lookupRoute(parsed.KeyHash)
	if err != nil {
		t.Fatalf("first lookup: %v", err)
	}

	_, err = s.lookupRoute(parsed.KeyHash)
	if err != nil {
		t.Fatalf("second lookup: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 API call, got %d (cache should prevent 2nd call)", callCount)
	}
}

func TestLookupRoute_CacheExpiry(t *testing.T) {
	hex64 := strings.Repeat("a", 64)
	parsed, _ := ParseTenantKey("sk_live_" + hex64 + "_rw")

	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
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

	_, err := s.lookupRoute(parsed.KeyHash)
	if err != nil {
		t.Fatalf("first lookup: %v", err)
	}

	entry, _ := s.routeCache.Load(parsed.KeyHash)
	cached := entry.(*TenantRoute)
	cached.CachedAt = time.Now().Add(-120 * time.Second)

	_, err = s.lookupRoute(parsed.KeyHash)
	if err != nil {
		t.Fatalf("second lookup after expiry: %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 API calls after cache expiry, got %d", callCount)
	}
}

func TestLookupRoute_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	s := NewServer(Config{
		Address:         ":0",
		ControlPlaneURL: ts.URL,
		GatewaySecret:   "test-secret",
	})

	_, err := s.lookupRoute("nonexistenthash")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}