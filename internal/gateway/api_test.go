package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"strings"
)

func TestFetchRoute_Success(t *testing.T) {
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
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Errorf("expected Authorization header 'Bearer test-secret', got %s", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept header 'application/json', got %s", r.Header.Get("Accept"))
		}
		json.NewEncoder(w).Encode(fixture)
	}))
	defer ts.Close()

	s := NewServer(Config{
		Address:         ":0",
		ControlPlaneURL: ts.URL,
		GatewaySecret:   "test-secret",
	})

	route, err := s.fetchRouteFromControlPlane(parsed.KeyHash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.TenantID != "project-test1" {
		t.Errorf("tenant_id=%s, want project-test1", route.TenantID)
	}
	if route.PgBouncerRWHost != "pgbouncer-test1-rw" {
		t.Errorf("rw_host=%s, want pgbouncer-test1-rw", route.PgBouncerRWHost)
	}
	if route.PgBouncerROHost != "pgbouncer-test1-ro" {
		t.Errorf("ro_host=%s, want pgbouncer-test1-ro", route.PgBouncerROHost)
	}
}

func TestFetchRoute_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	s := NewServer(Config{
		Address:         ":0",
		ControlPlaneURL: ts.URL,
		GatewaySecret:   "test-secret",
	})

	_, err := s.fetchRouteFromControlPlane("deadbeef")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestFetchRoute_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := NewServer(Config{
		Address:         ":0",
		ControlPlaneURL: ts.URL,
		GatewaySecret:   "test-secret",
	})

	_, err := s.fetchRouteFromControlPlane("deadbeef")
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}