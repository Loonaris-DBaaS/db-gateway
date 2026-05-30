package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const maxResponseBody = 1 << 20

func (s *Server) fetchRouteFromControlPlane(keyHash string) (*TenantRoute, error) {
	url := fmt.Sprintf("%s/internal/routes/%s", s.config.ControlPlaneURL, keyHash)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.config.GatewaySecret)
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("control plane request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("route not found: %s", keyHash)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("control plane returned HTTP %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxResponseBody)

	var route TenantRoute
	if err := json.NewDecoder(limited).Decode(&route); err != nil {
		return nil, fmt.Errorf("decode route: %w", err)
	}

	return &route, nil
}