package gateway

import (
	"crypto/sha256"
	"fmt"
	"regexp"
)

var keyPattern = regexp.MustCompile(`^sk_live_([a-f0-9]{64})_(rw|ro)$`)

type ParsedKey struct {
	BaseKey string
	KeyHash string
	Mode    string
}

func ParseTenantKey(raw string) (*ParsedKey, error) {
	matches := keyPattern.FindStringSubmatch(raw)
	if matches == nil {
		return nil, fmt.Errorf("invalid key format")
	}

	baseKey := matches[1]
	mode := matches[2]
	hashBytes := sha256.Sum256([]byte(baseKey))
	keyHash := fmt.Sprintf("%x", hashBytes)

	return &ParsedKey{
		BaseKey: baseKey,
		KeyHash: keyHash,
		Mode:    mode,
	}, nil
}