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
	idx     uint32
	set     map[string]struct{} // 去重用（记录是否存在）
}

func New() *Pool {
	return &Pool{set: make(map[string]struct{})}
}

// Add 追加一个代理；已存在则按需续期到更晚的过期时间
func (p *Pool) Add(addr string, ttl time.Duration) {
	if addr == "" || ttl <= 0 {
		return
	}
	exp := time.Now().Add(ttl)

	p.mu.Lock()
	defer p.mu.Unlock()

	// 已存在：续期
	if _, ok := p.set[addr]; ok {
		for i := range p.proxies {
			if p.proxies[i].Addr == addr {
				if exp.After(p.proxies[i].ExpireAt) {
					p.proxies[i].ExpireAt = exp
				}
				return
			}
		}
		// 理论上不会走到这里（set 有但切片里没找到），
		// 但为健壮性起见，继续走新增逻辑。
	}

	// 新增
	p.proxies = append(p.proxies, Proxy{Addr: addr, ExpireAt: exp})
	p.set[addr] = struct{}{}
}

// Get 轮询获取一个未过期的代理；会跳过过期项（不在此处删除，交给 Sweep/Remove）
func (p *Pool) Get() (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	n := len(p.proxies)
	if n == 0 {
		return "", false
	}
	now := time.Now()
	for tries := 0; tries < n; tries++ {
		i := int(atomic.AddUint32(&p.idx, 1)) % n
		pr := p.proxies[i]
		if now.Before(pr.ExpireAt) {
			return pr.Addr, true
		}
	}
	return "", false
}

// Remove 从池中移除一个代理；返回是否真的移除了（存在即删）
// 线程安全：使用写锁。
func (p *Pool) Remove(addr string) bool {
	if addr == "" {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.set[addr]; !ok {
		return false
	}
	// 在线性表中找到并删除；保持顺序（稳定删除）
	for i := range p.proxies {
		if p.proxies[i].Addr == addr {
			// 删除切片元素 i
			copy(p.proxies[i:], p.proxies[i+1:])
			p.proxies = p.proxies[:len(p.proxies)-1]
			delete(p.set, addr)

			// 调整 idx，避免越界（可选，属于健壮性处理）
			if len(p.proxies) == 0 {
				p.idx = 0
			} else {
				p.idx %= uint32(len(p.proxies))
			}
			return true
		}
	}
	// 异常：set 有但切片未找到 —— 纠正 set
	delete(p.set, addr)
	return false
}

// Delete 是 Remove 的同义方法，便于与外部调用保持兼容。
func (p *Pool) Delete(addr string) bool {
	return p.Remove(addr)
}

// Sweep 清理已过期项
func (p *Pool) Sweep() {
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()

	dst := p.proxies[:0]
	for _, pr := range p.proxies {
		if now.Before(pr.ExpireAt) {
			dst = append(dst, pr)
		} else {
			delete(p.set, pr.Addr)
		}
	}
	p.proxies = dst
	// idx 修正（防止越界）
	if len(p.proxies) == 0 {
		p.idx = 0
	} else {
		p.idx %= uint32(len(p.proxies))
	}
}

func (p *Pool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.proxies)
}
