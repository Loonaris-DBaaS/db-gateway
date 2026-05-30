package gateway

import (
	"time"
)

type TenantRoute struct {
	TenantID        string `json:"tenant_id"`
	PgBouncerRWHost string `json:"pgbouncer_rw_host"`
	PgBouncerRWPort int    `json:"pgbouncer_rw_port"`
	PgBouncerROHost string `json:"pgbouncer_ro_host"`
	PgBouncerROPort int    `json:"pgbouncer_ro_port"`
	Status          string `json:"status"`
	CachedAt        time.Time
}

const cacheTTL = 60 * time.Second

func (s *Server) lookupRoute(keyHash string) (*TenantRoute, error) {
	if entry, ok := s.routeCache.Load(keyHash); ok {
		route := entry.(*TenantRoute)
		if time.Since(route.CachedAt) <= cacheTTL {
			return route, nil
		}
		s.routeCache.Delete(keyHash)
	}

	route, err, _ := s.sfGroup.Do(keyHash, func() (any, error) {
		route, fetchErr := s.fetchRouteFromControlPlane(keyHash)
		if fetchErr != nil {
			return nil, fetchErr
		}
		route.CachedAt = time.Now()
		s.routeCache.Store(keyHash, route)
		return route, nil
	})
	if err != nil {
		return nil, err
	}
	return route.(*TenantRoute), nil
}