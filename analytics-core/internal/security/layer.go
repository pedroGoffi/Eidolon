package security

import (
	"net/http"

	"eidolon/analytics-core/internal/models"
)

// Layer é o Security Layer descrito no blueprint: recebe os dados do
// request já extraídos pelo Gateway e devolve uma decisão única.
// A ordem de avaliação importa (do mais barato/decisivo pro mais caro):
// blacklist -> rate limit -> WAF -> anomaly detection.
type Layer struct {
	Blacklist *Blacklist
	RateLimit *RateLimiter
	WAF       *WAF
	Anomaly   *AnomalyDetector
}

func NewLayer(bl *Blacklist, rl *RateLimiter, waf *WAF, an *AnomalyDetector) *Layer {
	return &Layer{Blacklist: bl, RateLimit: rl, WAF: waf, Anomaly: an}
}

// Evaluate aplica todas as checagens de segurança. `routeKey` deve
// identificar a rota para rate limit por rota (ex: subdomain+path).
func (l *Layer) Evaluate(ip, routeKey, pathAndQuery, body string) models.SecurityDecision {
	if l.Blacklist.IsBanned(ip) {
		return models.SecurityDecision{
			Allowed:    false,
			Reason:     "blacklist",
			HTTPStatus: http.StatusForbidden,
		}
	}

	if !l.RateLimit.Allow(ip) || !l.RateLimit.Allow(ip + ":" + routeKey) {
		return models.SecurityDecision{
			Allowed:    false,
			Reason:     "rate_limit",
			HTTPStatus: http.StatusTooManyRequests,
		}
	}

	if rule := l.WAF.Inspect(pathAndQuery, body); rule != nil && rule.Severity == "block" {
		return models.SecurityDecision{
			Allowed:    false,
			Reason:     "waf",
			RuleID:     rule.ID,
			HTTPStatus: http.StatusForbidden,
		}
	}

	// Spike detection não bloqueia por padrão aqui — apenas sinaliza.
	// Em produção, um spike confirmado pode disparar ban automático
	// temporário via Blacklist.Ban(ip, curta_duração) a partir de uma
	// política configurável no painel.
	_ = l.Anomaly.Record(ip)

	return models.SecurityDecision{Allowed: true, HTTPStatus: http.StatusOK}
}
