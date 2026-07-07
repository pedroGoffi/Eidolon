// Package router implementa o Decision Engine: resolve subdomínio+path
// para o serviço interno de destino, com suporte a hot reload de regras
// sem downtime (troca atômica de um snapshot imutável).
package router

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"

	"eidolon/analytics-core/internal/models"
)

var ErrNoRoute = errors.New("nenhuma rota encontrada para subdomain+path")

// Engine resolve rotas. O snapshot de regras é trocado atomicamente via
// atomic.Pointer, então leituras no caminho quente nunca bloqueiam em
// lock — Reload() é a única operação "cara".
type Engine struct {
	mu    sync.Mutex // protege escritas (Reload/Upsert/Delete) entre si
	rules atomic.Pointer[[]models.RoutingRule]
}

func NewEngine() *Engine {
	e := &Engine{}
	empty := []models.RoutingRule{}
	e.rules.Store(&empty)
	return e
}

// Reload substitui o conjunto de regras (chamado periodicamente a partir
// do Postgres, ou via evento do painel administrativo).
func (e *Engine) Reload(rules []models.RoutingRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.store(rules)
}

// List retorna todas as regras atuais, ordenadas por prioridade — usado
// pelo painel administrativo (tela de rotas/portas).
func (e *Engine) List() []models.RoutingRule {
	rules := *e.rules.Load()
	out := make([]models.RoutingRule, len(rules))
	copy(out, rules)
	return out
}

// Upsert cria ou atualiza uma regra (match por ID) e recarrega o cache
// atomicamente. Usado pelo painel para configurar subdomínio/rota/porta
// de destino sem reiniciar o Core.
func (e *Engine) Upsert(rule models.RoutingRule) {
	e.mu.Lock()
	defer e.mu.Unlock()

	current := *e.rules.Load()
	updated := make([]models.RoutingRule, 0, len(current)+1)
	found := false
	for _, r := range current {
		if r.ID == rule.ID {
			updated = append(updated, rule)
			found = true
		} else {
			updated = append(updated, r)
		}
	}
	if !found {
		updated = append(updated, rule)
	}
	e.store(updated)
}

// Delete remove uma regra por ID.
func (e *Engine) Delete(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	current := *e.rules.Load()
	updated := make([]models.RoutingRule, 0, len(current))
	for _, r := range current {
		if r.ID != id {
			updated = append(updated, r)
		}
	}
	e.store(updated)
}

// store ordena por prioridade desc e publica o novo snapshot. Chamador
// deve segurar e.mu.
func (e *Engine) store(rules []models.RoutingRule) {
	sorted := make([]models.RoutingRule, len(rules))
	copy(sorted, rules)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].Priority > sorted[j-1].Priority; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	e.rules.Store(&sorted)
}

// Resolve encontra o serviço de destino para um subdomínio+path.
// PathPattern suporta um wildcard simples de sufixo: "/api/*".
func (e *Engine) Resolve(subdomain, path string) (models.RoutingRule, error) {
	rules := *e.rules.Load()
	for _, r := range rules {
		if !r.Enabled || r.Subdomain != subdomain {
			continue
		}
		if matchPath(r.PathPattern, path) {
			return r, nil
		}
	}
	return models.RoutingRule{}, ErrNoRoute
}

func matchPath(pattern, path string) bool {
	// Wildcard global
	if pattern == "*" {
		return true
	}
	// Wildcard de sufixo: /api/*
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(path, prefix)
	}
	// Wildcard de prefixo: *.js
	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*.")
		return strings.HasSuffix(path, suffix)
	}
	return pattern == path
}
