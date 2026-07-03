// Package models define as estruturas de dados centrais do Eidolon.
package models

import "time"

// RequestLog representa o registro completo de uma requisição processada
// pelo Analytics Core. É o que acaba persistido no ClickHouse.
type RequestLog struct {
	CorrelationID     string    `json:"correlation_id"`
	Timestamp         time.Time `json:"timestamp"`
	IP                string    `json:"ip"`
	UserAgent         string    `json:"user_agent"`
	Method            string    `json:"method"`
	Path              string    `json:"path"`
	Subdomain         string    `json:"subdomain"`
	BodySize          int64     `json:"body_size"`
	StatusCode        int       `json:"status_code"`
	LatencyMs         int64     `json:"latency_ms"`
	ServiceTarget     string    `json:"service_target"`
	SecurityDecision  string    `json:"security_decision"` // allowed | blocked_waf | blocked_rate_limit | blocked_blacklist
	WAFRuleMatched    string    `json:"waf_rule_matched,omitempty"`
}

// RoutingRule descreve como um request deve ser roteado para um serviço interno.
type RoutingRule struct {
	ID                 string `json:"id"`
	Subdomain          string `json:"subdomain"`
	PathPattern        string `json:"path_pattern"` // ex: "/api/*"
	DestinationService string `json:"destination_service"`
	DestinationAddr    string `json:"destination_addr"` // ex: "127.0.0.1:9001"
	Priority           int    `json:"priority"`
	Enabled            bool   `json:"enabled"`
}

// SecurityRuleType enumera os tipos de regra de segurança suportados.
type SecurityRuleType string

const (
	RuleBlacklist SecurityRuleType = "blacklist"
	RuleWhitelist SecurityRuleType = "whitelist"
	RuleRateLimit SecurityRuleType = "rate_limit"
	RuleWAF       SecurityRuleType = "waf_pattern"
)

// SecurityRule descreve uma regra de segurança dinâmica (Postgres em produção).
type SecurityRule struct {
	ID        string           `json:"id"`
	Type      SecurityRuleType `json:"rule_type"`
	Target    string           `json:"target"` // IP, CIDR, rota, ou regex
	Enabled   bool             `json:"enabled"`
	ExpiresAt *time.Time       `json:"expires_at,omitempty"`
}

// SecurityDecision é o resultado da avaliação do Security Layer.
type SecurityDecision struct {
	Allowed    bool
	Reason     string // "blacklist" | "rate_limit" | "waf" | ""
	RuleID     string
	HTTPStatus int
}
