package security

import (
	"regexp"
	"sync"
)

// WAFRule é uma regra de detecção de payload malicioso.
type WAFRule struct {
	ID       string
	Name     string
	Pattern  *regexp.Regexp
	Severity string // "log_only" | "challenge" | "block"
}

// WAF avalia path, query string e (opcionalmente) body contra um conjunto
// de regras. Regras são recarregáveis em tempo real (hot reload a partir
// do Postgres, via SetRules) sem downtime — troca atômica do slice.
type WAF struct {
	mu    sync.RWMutex
	rules []WAFRule
}

// NewWAF cria o WAF já com um conjunto básico de regras conhecidas.
// Em produção, este conjunto inicial deve ser complementado/substituído
// pelas regras configuradas no painel (tabela security_rules).
func NewWAF() *WAF {
	w := &WAF{}
	w.SetRules(defaultRules())
	return w
}

func defaultRules() []WAFRule {
	return []WAFRule{
		{
			ID:       "waf-sqli-1",
			Name:     "SQL Injection - union/select",
			Pattern:  regexp.MustCompile(`(?i)(\bunion\b.{1,40}\bselect\b|\bor\b\s+1=1|;\s*drop\s+table)`),
			Severity: "block",
		},
		{
			ID:       "waf-xss-1",
			Name:     "XSS - script tag / event handler",
			Pattern:  regexp.MustCompile(`(?i)(<script[\s>]|on(error|load|click)\s*=|javascript:)`),
			Severity: "block",
		},
		{
			ID:       "waf-traversal-1",
			Name:     "Path Traversal",
			Pattern:  regexp.MustCompile(`(\.\./|\.\.\\|%2e%2e%2f)`),
			Severity: "block",
		},
		{
			ID:       "waf-rfi-lfi-1",
			Name:     "RFI/LFI - wrapper suspeito",
			Pattern:  regexp.MustCompile(`(?i)(php://|file://|data://|expect://)`),
			Severity: "block",
		},
	}
}

// SetRules substitui o conjunto de regras atomicamente (hot reload).
func (w *WAF) SetRules(rules []WAFRule) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.rules = rules
}

// Inspect avalia path+query (e opcionalmente body) contra as regras.
// Retorna a primeira regra que bateu, ou nil se limpo.
func (w *WAF) Inspect(pathAndQuery string, body string) *WAFRule {
	w.mu.RLock()
	rules := w.rules
	w.mu.RUnlock()

	for i := range rules {
		if rules[i].Pattern.MatchString(pathAndQuery) {
			return &rules[i]
		}
		if body != "" && rules[i].Pattern.MatchString(body) {
			return &rules[i]
		}
	}
	return nil
}
