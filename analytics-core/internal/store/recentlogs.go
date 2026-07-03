// Package store mantém estruturas de dados em memória que o painel
// administrativo consulta diretamente (sem precisar do ClickHouse) —
// hoje só o histórico recente de requests. Quando o ClickHouse real
// entrar, o Request Inspector do painel passa a consultar um endpoint
// que faz a query lá, e este ring buffer vira só um cache de "últimos
// eventos" para o carregamento inicial da tela.
package store

import (
	"sync"

	"eidolon/analytics-core/internal/models"
)

type RecentLogs struct {
	mu       sync.RWMutex
	buf      []models.RequestLog
	capacity int
	next     int
	filled   bool
}

func NewRecentLogs(capacity int) *RecentLogs {
	return &RecentLogs{
		buf:      make([]models.RequestLog, capacity),
		capacity: capacity,
	}
}

func (r *RecentLogs) Add(entry models.RequestLog) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.next] = entry
	r.next = (r.next + 1) % r.capacity
	if r.next == 0 {
		r.filled = true
	}
}

// List retorna as últimas `limit` entradas, mais recentes primeiro.
func (r *RecentLogs) List(limit int) []models.RequestLog {
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := r.next
	if r.filled {
		total = r.capacity
	}
	if limit <= 0 || limit > total {
		limit = total
	}

	out := make([]models.RequestLog, 0, limit)
	idx := r.next - 1
	for i := 0; i < limit; i++ {
		if idx < 0 {
			idx = r.capacity - 1
		}
		out = append(out, r.buf[idx])
		idx--
	}
	return out
}
