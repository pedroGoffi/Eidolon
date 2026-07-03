package security

import (
	"sync"
	"time"
)

// Blacklist mantém IPs bloqueados em memória, com TTL opcional.
//
// Em produção (múltiplas réplicas do Core), isto deve virar Redis
// (SET com EXPIRE), para que um ban aplicado pelo painel em uma réplica
// tenha efeito imediato em todas. A interface abaixo já isola essa troca.
type Blacklist struct {
	mu      sync.RWMutex
	entries map[string]time.Time // ip -> expira_em (zero = permanente)
}

func NewBlacklist() *Blacklist {
	return &Blacklist{entries: make(map[string]time.Time)}
}

// Ban bloqueia um IP. duration == 0 significa bloqueio permanente
// (até remoção manual via painel).
func (b *Blacklist) Ban(ip string, duration time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if duration == 0 {
		b.entries[ip] = time.Time{}
	} else {
		b.entries[ip] = time.Now().Add(duration)
	}
}

func (b *Blacklist) Unban(ip string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.entries, ip)
}

// IsBanned verifica se o IP está bloqueado. Entradas expiradas são
// removidas de forma preguiçosa (lazy) no momento da checagem.
func (b *Blacklist) IsBanned(ip string) bool {
	b.mu.RLock()
	expiresAt, found := b.entries[ip]
	b.mu.RUnlock()

	if !found {
		return false
	}
	if expiresAt.IsZero() {
		return true // ban permanente
	}
	if time.Now().After(expiresAt) {
		b.Unban(ip)
		return false
	}
	return true
}

// Snapshot retorna a lista de IPs banidos atualmente — usado pelo
// painel administrativo (Security Dashboard).
func (b *Blacklist) Snapshot() map[string]time.Time {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make(map[string]time.Time, len(b.entries))
	for k, v := range b.entries {
		out[k] = v
	}
	return out
}
