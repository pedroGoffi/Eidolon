// Package gateway implementa o entrypoint HTTP do Eidolon Analytics Core:
// recebe 100% do tráfego vindo do Nginx e orquestra todo o fluxo descrito
// no blueprint (log -> security -> routing -> proxy -> log final -> evento
// realtime).
package gateway

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"eidolon/analytics-core/internal/idgen"
	"eidolon/analytics-core/internal/logger"
	"eidolon/analytics-core/internal/models"
	"eidolon/analytics-core/internal/realtime"
	"eidolon/analytics-core/internal/router"
	"eidolon/analytics-core/internal/security"
	"eidolon/analytics-core/internal/store"
)

// MaxInspectBodyBytes limita quanto do body é lido para inspeção do WAF.
// Ler o body inteiro de uploads grandes só para regex seria caro e
// desnecessário — os primeiros KB já capturam a esmagadora maioria dos
// payloads maliciosos típicos (SQLi/XSS/etc em JSON/form bodies).
const MaxInspectBodyBytes = 64 * 1024

type Handler struct {
	Security   *security.Layer
	Router     *router.Engine
	Sink       logger.Sink
	Hub        *realtime.Hub
	RecentLogs *store.RecentLogs
	OpLog      *slog.Logger

	UpstreamTimeout time.Duration

	proxyCacheMu sync.Mutex
	proxyCache   map[string]*httputil.ReverseProxy
}

func NewHandler(sec *security.Layer, eng *router.Engine, sink logger.Sink, hub *realtime.Hub, recentLogs *store.RecentLogs, opLog *slog.Logger, upstreamTimeout time.Duration) *Handler {
	return &Handler{
		Security:        sec,
		Router:          eng,
		Sink:            sink,
		Hub:             hub,
		RecentLogs:      recentLogs,
		OpLog:           opLog,
		UpstreamTimeout: upstreamTimeout,
		proxyCache:      make(map[string]*httputil.ReverseProxy),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	correlationID := idgen.New()
	ctx := withCorrelationID(r.Context(), correlationID)
	r = r.WithContext(ctx)

	ip := clientIP(r)
	subdomain := extractSubdomain(r.Host)

	// Lê uma amostra do body para inspeção do WAF, sem consumir o
	// body original de forma irreversível (precisa seguir pro upstream).
	var bodySample string
	var bodySize int64
	if r.Body != nil {
		limited := io.LimitReader(r.Body, MaxInspectBodyBytes+1)
		buf, _ := io.ReadAll(limited)
		bodySample = string(buf)
		// Reconstrói o body para o proxy repassar ao serviço interno.
		rest, _ := io.ReadAll(r.Body)
		full := append(buf, rest...)
		bodySize = int64(len(full))
		r.Body = io.NopCloser(bytes.NewReader(full))
	}

	// Decodifica a query string para inspeção do WAF — payloads maliciosos
	// costumam vir URL-encoded (%20, %27 etc.) justamente para escapar de
	// regras que só olham a string crua. QueryUnescape falha em silêncio
	// (usa o valor bruto) caso o encoding esteja malformado.
	decodedQuery := r.URL.RawQuery
	if unescaped, err := url.QueryUnescape(r.URL.RawQuery); err == nil {
		decodedQuery = unescaped
	}
	pathAndQuery := r.URL.Path
	if decodedQuery != "" {
		pathAndQuery += "?" + decodedQuery
	}

	entry := models.RequestLog{
		CorrelationID: correlationID,
		Timestamp:     start,
		IP:            ip,
		UserAgent:     r.UserAgent(),
		Method:        r.Method,
		Path:          r.URL.Path,
		Subdomain:     subdomain,
		BodySize:      bodySize,
	}

	// --- Security Layer (fail secure: erro aqui = bloqueio, nunca bypass) ---
	decision := h.Security.Evaluate(ip, subdomain+r.URL.Path, pathAndQuery, bodySample)
	if !decision.Allowed {
		entry.StatusCode = decision.HTTPStatus
		entry.SecurityDecision = "blocked_" + decision.Reason
		entry.WAFRuleMatched = decision.RuleID
		entry.LatencyMs = time.Since(start).Milliseconds()

		w.WriteHeader(decision.HTTPStatus)
		h.finalize(entry)
		return
	}

	// --- Decision Engine ---
	rule, err := h.Router.Resolve(subdomain, r.URL.Path)
	if err != nil {
		entry.StatusCode = http.StatusNotFound
		entry.SecurityDecision = "allowed"
		entry.LatencyMs = time.Since(start).Milliseconds()
		w.WriteHeader(http.StatusNotFound)
		h.finalize(entry)
		return
	}
	entry.ServiceTarget = rule.DestinationService
	entry.SecurityDecision = "allowed"

	// --- Proxy interno ---
	proxy := h.getProxy(rule.DestinationAddr)
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	proxy.ServeHTTP(rec, r)

	entry.StatusCode = rec.status
	entry.LatencyMs = time.Since(start).Milliseconds()
	h.finalize(entry)
}

// finalize persiste o log final (assíncrono) e publica um evento
// agregável para o painel em tempo real.
func (h *Handler) finalize(entry models.RequestLog) {
	h.Sink.Write(entry)
	h.RecentLogs.Add(entry)
	h.Hub.Broadcast(realtime.Event{
		Type: "request",
		Data: entry,
	})
}

func (h *Handler) getProxy(destAddr string) *httputil.ReverseProxy {
	h.proxyCacheMu.Lock()
	defer h.proxyCacheMu.Unlock()

	if p, ok := h.proxyCache[destAddr]; ok {
		return p
	}
	p := newReverseProxy(destAddr, h.UpstreamTimeout)
	h.proxyCache[destAddr] = p
	return p
}

// clientIP extrai o IP real do socket TCP — nunca de headers X-Forwarded-*
// enviados pelo cliente. O Nginx, como única borda pública, é responsável
// por popular corretamente o que chega aqui via proxy_set_header
// controlado (ver blueprint seção 7).
func clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		return host[:idx]
	}
	return host
}

// extractSubdomain extrai o subdomínio do Host header.
// ex: "loja.merchonline.com.br" -> "loja"
func extractSubdomain(host string) string {
	host = strings.Split(host, ":")[0] // remove porta, se houver
	parts := strings.Split(host, ".")
	if len(parts) <= 2 {
		return "" // domínio raiz, sem subdomínio
	}
	return parts[0]
}

// statusRecorder captura o status code final da resposta do proxy,
// necessário para o log estruturado (o ReverseProxy não expõe isso
// diretamente).
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

var _ = context.Background // mantém import context usado por withCorrelationID
