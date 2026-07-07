// Eidolon Analytics Core — entrypoint do servidor.
//
// Este processo é o único ponto que o Nginx alcança (via localhost).
// Ele recebe todo o tráfego, aplica segurança (blacklist/rate limit/WAF),
// resolve a rota de destino e faz proxy para o serviço interno correto,
// registrando logs estruturados e emitindo eventos em tempo real para o
// painel administrativo.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eidolon/analytics-core/internal/admin"
	"eidolon/analytics-core/internal/config"
	"eidolon/analytics-core/internal/gateway"
	"eidolon/analytics-core/internal/logger"
	"eidolon/analytics-core/internal/models"
	"eidolon/analytics-core/internal/realtime"
	"eidolon/analytics-core/internal/router"
	"eidolon/analytics-core/internal/security"
	"eidolon/analytics-core/internal/store"
)

func main() {
	cfg := config.Load()
	opLog := logger.New(cfg.LogLevel)

	// --- Security Layer ---
	blacklist := security.NewBlacklist()
	rateLimiter := security.NewRateLimiter(cfg.RateLimitWindow, cfg.RateLimitMax)
	waf := security.NewWAF()
	anomaly := security.NewAnomalyDetector(cfg.RateLimitWindow, 30, 5.0)
	secLayer := security.NewLayer(blacklist, rateLimiter, waf, anomaly)

	// --- Decision Engine ---
	engine := router.NewEngine()
	loadInitialRoutes(engine) // placeholder — em produção vem do Postgres
	go hotReloadLoop(engine, cfg.RulesReloadInterval, opLog)

	// --- Logging assíncrono (troque StdoutWriter por um writer real de
	// ClickHouse quando tiver o cliente disponível) ---
	sink := logger.NewAsyncSink(4096, opLog, logger.StdoutWriter)
	defer sink.Close()

	// --- Realtime hub (SSE) ---
	hub := realtime.NewHub()

	// --- Histórico recente de logs (Request Inspector do painel) ---
	recentLogs := store.NewRecentLogs(1000)

	// --- Handler principal ---
	h := gateway.NewHandler(secLayer, engine, sink, hub, recentLogs, opLog, cfg.UpstreamTimeout)

	// --- API administrativa (painel Next.js) ---
	adminAPI := admin.NewAPI(blacklist, engine, recentLogs, cfg.AdminToken)
	if cfg.AdminToken == "" {
		opLog.Warn("ADMIN_TOKEN não definido — API administrativa está sem autenticação, use isso apenas em desenvolvimento local")
	}

	mux := http.NewServeMux()
	adminAPI.Register(mux)
	// Endpoint de eventos em tempo real para o painel administrativo.
	mux.HandleFunc("/__eidolon/realtime", hub.ServeHTTP)
	mux.HandleFunc("/__eidolon/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	// Catch-all: qualquer coisa que não seja /api/* ou /__eidolon/* é
	// tráfego de verdade, vai pro fluxo completo do gateway.
	mux.Handle("/", h)

	srv := &http.Server{
		Addr:         cfg.BindAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: cfg.UpstreamTimeout + 5*time.Second,
		IdleTimeout:  60 * time.Second,
	}

	opLog.Info("eidolon analytics-core iniciando", "bind_addr", cfg.BindAddr)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			opLog.Error("falha ao iniciar servidor", "error", err.Error())
			os.Exit(1)
		}
	}()

	// Graceful shutdown: drena requests em andamento e fecha o sink de logs.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	opLog.Info("encerrando eidolon analytics-core...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		opLog.Error("erro no shutdown gracioso", "error", err.Error())
	}
}

// loadInitialRoutes popula algumas rotas de exemplo. Em produção, isto
// vem de uma query ao Postgres na inicialização.
func loadInitialRoutes(engine *router.Engine) {
	engine.Reload([]models.RoutingRule{
		{
			ID:                 "smart-work-force",
			Subdomain:          "workon",
			PathPattern:        "*",
			DestinationService: "smartworkforce",
			DestinationAddr:    "127.0.0.1:3000",
			Priority:           10,
			Enabled:            true,
		},
	})
}

// hotReloadLoop simula o hot reload periódico a partir do Postgres.
// Em produção: query real + comparação de versão/hash para só recarregar
// quando algo mudou, e/ou trigger via evento do painel (mais responsivo
// que polling).
func hotReloadLoop(engine *router.Engine, interval time.Duration, opLog *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		// TODO produção: SELECT * FROM routing_rules WHERE enabled = true
		// ORDER BY priority DESC; engine.Reload(rows)
		opLog.Debug("hot reload de rotas executado (placeholder, sem Postgres conectado)")
	}
}
