package gateway

import (
	"strings"
	"testing"
)

func TestParseTenantKey(t *testing.T) {
	hex64 := strings.Repeat("a", 64)
	validRW := "sk_live_" + hex64 + "_rw"
	validRO := "sk_live_" + hex64 + "_ro"

	tests := []struct {
		name     string
		input    string
		wantErr  bool
		wantMode string
	}{
		{"valid_rw", validRW, false, "rw"},
		{"valid_ro", validRO, false, "ro"},
		{"missing_prefix", "live_" + hex64 + "_rw", true, ""},
		{"too_short_hex", "sk_live_aabb_rw", true, ""},
		{"no_mode_suffix", "sk_live_" + hex64, true, ""},
		{"invalid_mode", "sk_live_" + hex64 + "_xx", true, ""},
		{"empty_string", "", true, ""},
		{"uppercase_hex", "sk_live_" + strings.Repeat("A", 64) + "_rw", true, ""},
		{"mixed_case_hex", "sk_live_" + strings.Repeat("aB", 32) + "_rw", true, ""},
		{"mode_rwx", "sk_live_" + hex64 + "_rwx", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTenantKey(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Mode != tt.wantMode {
				t.Errorf("mode=%q, want=%q", got.Mode, tt.wantMode)
			}
			if len(got.KeyHash) != 64 {
				t.Errorf("keyHash length=%d, want 64", len(got.KeyHash))
			}
			if len(got.BaseKey) != 64 {
				t.Errorf("baseKey length=%d, want 64", len(got.BaseKey))
			}
		})
	}
}

func TestParseTenantKey_SHA256Consistency(t *testing.T) {
	hex64 := strings.Repeat("a", 64)
	key1, _ := ParseTenantKey("sk_live_" + hex64 + "_rw")
	key2, _ := ParseTenantKey("sk_live_" + hex64 + "_ro")

	if key1.KeyHash != key2.KeyHash {
		t.Errorf("same base key should produce same hash, got %s vs %s", key1.KeyHash, key2.KeyHash)
	}

	hex64b := strings.Repeat("b", 64)
	key3, _ := ParseTenantKey("sk_live_" + hex64b + "_rw")
	if key1.KeyHash == key3.KeyHash {
		t.Error("different base keys should produce different hashes")
	}
}

func TestParseTenantKey_E2ETestHashes(t *testing.T) {
	tenant1Base := strings.Repeat("a", 64)
	tenant2Base := strings.Repeat("b", 64)
	tenant3Base := strings.Repeat("c", 64)

	expected := map[string]string{
		tenant1Base: "ffe054fe7ae0cb6dc65c3af9b61d5209f439851db43d0ba5997337df154668eb",
		tenant2Base: "a0fab1377f49a759b57f63318262ebe89fabfc990e8e93ceac2984561482b9d4",
		tenant3Base: "52b6419d27bd7f547cee3b92f8c17a908b8a49601ecbec161e5030de1dfe9e0a",
	}

	for base, wantHash := range expected {
		parsed, err := ParseTenantKey("sk_live_" + base + "_rw")
		if err != nil {
			t.Fatalf("ParseTenantKey(%s): %v", base[:8], err)
		}
		if parsed.KeyHash != wantHash {
			t.Errorf("hash mismatch for base=%s...\ngot:  %s\nwant: %s", base[:8], parsed.KeyHash, wantHash)
		}
	}
}