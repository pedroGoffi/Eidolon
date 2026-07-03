package security

import (
	"sync"
	"time"
)

// RateLimiter implementa limitação por janela fixa, por chave (ex: "ip",
// ou "ip:rota" para limites por rota). Simples e barato — para produção
// multi-réplica, trocar por Redis (INCR + EXPIRE) via a mesma interface.
type RateLimiter struct {
	mu      sync.Mutex
	window  time.Duration
	max     int
	buckets map[string]*bucket
}

type bucket struct {
	count     int
	windowEnd time.Time
}

func NewRateLimiter(window time.Duration, max int) *RateLimiter {
	rl := &RateLimiter{
		window:  window,
		max:     max,
		buckets: make(map[string]*bucket),
	}
	go rl.gcLoop()
	return rl
}

// Allow registra uma tentativa para `key` e retorna false se o limite
// da janela atual foi excedido.
func (rl *RateLimiter) Allow(key string) bool {
	return rl.AllowN(key, rl.max)
}

// AllowN permite sobrescrever o limite máximo por chamada (ex: limites
// diferentes por rota, vindos de SecurityRule.config).
func (rl *RateLimiter) AllowN(key string, max int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.buckets[key]
	if !exists || now.After(b.windowEnd) {
		rl.buckets[key] = &bucket{count: 1, windowEnd: now.Add(rl.window)}
		return true
	}

	b.count++
	return b.count <= max
}

// gcLoop remove buckets expirados periodicamente para não vazar memória
// sob alta cardinalidade de IPs.
func (rl *RateLimiter) gcLoop() {
	ticker := time.NewTicker(rl.window * 10)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		rl.mu.Lock()
		for k, b := range rl.buckets {
			if now.After(b.windowEnd) {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}
