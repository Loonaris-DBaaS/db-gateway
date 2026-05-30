package gateway

import "sync"

type singleflightGroup struct {
	mu    sync.Mutex
	calls map[string]*call
}

type call struct {
	wg   sync.WaitGroup
	val  any
	err  error
}

func (g *singleflightGroup) Do(key string, fn func() (any, error)) (any, error, bool) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[string]*call)
	}
	if c, ok := g.calls[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err, false
	}
	c := &call{}
	c.wg.Add(1)
	g.calls[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()

	return c.val, c.err, true
}