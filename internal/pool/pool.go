// placeholder pool.go
package pool

import (
	"sync"
	"sync/atomic"
	"time"
)

type Proxy struct {
	Addr     string
	ExpireAt time.Time
}

type Pool struct {
	mu      sync.RWMutex
	proxies []Proxy
	idx     uint32 // 轮询索引
}

func New() *Pool { return &Pool{} }

func (p *Pool) Set(addrs []string, ttl time.Duration) {
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	ps := make([]Proxy, 0, len(addrs))
	for _, a := range addrs {
		if a == "" {
			continue
		}
		ps = append(ps, Proxy{Addr: a, ExpireAt: now.Add(ttl)})
	}
	p.proxies = ps
}

func (p *Pool) Clean() {
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	dst := p.proxies[:0]
	for _, pr := range p.proxies {
		if now.Before(pr.ExpireAt) {
			dst = append(dst, pr)
		}
	}
	p.proxies = dst
}

func (p *Pool) Get() (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	n := len(p.proxies)
	if n == 0 {
		return "", false
	}
	i := atomic.AddUint32(&p.idx, 1)
	pr := p.proxies[int(i)%n]
	if time.Now().After(pr.ExpireAt) {
		return "", false
	}
	return pr.Addr, true
}
