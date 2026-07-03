// Package config carrega a configuração do Eidolon a partir de variáveis
// de ambiente. Em produção, regras dinâmicas (routing/security) vêm do
// Postgres com hot reload — isto aqui é só o bootstrap do processo.
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Endereço de bind do Core. Em produção deve ser 127.0.0.1 — o Core
	// nunca deve escutar em 0.0.0.0, pois só o Nginx pode alcançá-lo.
	BindAddr string

	// Timeouts do caminho crítico. Fail Secure: se qualquer dependência
	// estourar o timeout, o request é bloqueado (503), nunca liberado.
	SecurityLayerTimeout time.Duration
	DecisionEngineTimeout time.Duration
	UpstreamTimeout        time.Duration

	// Janela e limite padrão de rate limiting (por IP, por rota).
	RateLimitWindow time.Duration
	RateLimitMax    int

	// Intervalo de hot reload das regras de roteamento/segurança.
	RulesReloadInterval time.Duration

	// Token Bearer exigido nas ações de escrita da API administrativa
	// (banir IP, criar/editar/remover rota). Vazio = sem auth (dev only).
	AdminToken string

	LogLevel string
}

func Load() Config {
	return Config{
		BindAddr:               getEnv("BIND_ADDR", "127.0.0.1:8081"),
		SecurityLayerTimeout:   getDuration("SECURITY_LAYER_TIMEOUT_MS", 30*time.Millisecond),
		DecisionEngineTimeout:  getDuration("DECISION_ENGINE_TIMEOUT_MS", 20*time.Millisecond),
		UpstreamTimeout:        getDuration("UPSTREAM_TIMEOUT_MS", 5000*time.Millisecond),
		RateLimitWindow:        getDuration("RATE_LIMIT_WINDOW_MS", 1000*time.Millisecond),
		RateLimitMax:           getInt("RATE_LIMIT_MAX", 50),
		RulesReloadInterval:    getDuration("RULES_RELOAD_INTERVAL_MS", 5000*time.Millisecond),
		AdminToken:             getEnv("ADMIN_TOKEN", ""),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Millisecond
		}
	}
	return fallback
}
