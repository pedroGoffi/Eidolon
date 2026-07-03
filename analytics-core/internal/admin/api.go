// Package admin expõe a API REST consumida pelo painel administrativo
// Next.js: request inspector (logs), security dashboard (ban/unban IP)
// e configuração de rotas/portas (routing rules).
//
// Autenticação: um Bearer token simples via env var ADMIN_TOKEN, checado
// em todo request de escrita (POST/DELETE). Em produção isso deve virar
// sessão real com RBAC (ver blueprint seção 4) — este token único é o
// suficiente para uma primeira versão rodando na sua VPS.
package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"eidolon/analytics-core/internal/idgen"
	"eidolon/analytics-core/internal/models"
	"eidolon/analytics-core/internal/router"
	"eidolon/analytics-core/internal/security"
	"eidolon/analytics-core/internal/store"
)

type API struct {
	Blacklist  *security.Blacklist
	Router     *router.Engine
	RecentLogs *store.RecentLogs
	Token      string // se vazio, autenticação fica desabilitada (dev only)
}

func NewAPI(bl *security.Blacklist, eng *router.Engine, logs *store.RecentLogs, token string) *API {
	return &API{Blacklist: bl, Router: eng, RecentLogs: logs, Token: token}
}

// Register monta as rotas no mux fornecido (padrões de path do Go 1.22+,
// com método HTTP embutido, ex: "GET /api/logs").
func (a *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/logs", a.withCORS(a.handleListLogs))

	mux.HandleFunc("GET /api/security/blacklist", a.withCORS(a.handleListBans))
	mux.HandleFunc("POST /api/security/blacklist", a.withCORS(a.requireAuth(a.handleBan)))
	mux.HandleFunc("DELETE /api/security/blacklist/{ip}", a.withCORS(a.requireAuth(a.handleUnban)))

	mux.HandleFunc("GET /api/routing/rules", a.withCORS(a.handleListRules))
	mux.HandleFunc("POST /api/routing/rules", a.withCORS(a.requireAuth(a.handleUpsertRule)))
	mux.HandleFunc("DELETE /api/routing/rules/{id}", a.withCORS(a.requireAuth(a.handleDeleteRule)))

	// Preflight CORS para todas as rotas acima.
	mux.HandleFunc("OPTIONS /api/", a.withCORS(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
}

// --- middlewares -----------------------------------------------------

func (a *API) withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Em produção, restrinja a origem exata do painel em vez de "*".
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		next(w, r)
	}
}

func (a *API) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.Token == "" {
			next(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+a.Token {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// --- logs --------------------------------------------------------------

func (a *API) handleListLogs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	writeJSON(w, http.StatusOK, a.RecentLogs.List(limit))
}

// --- blacklist -----------------------------------------------------------

func (a *API) handleListBans(w http.ResponseWriter, r *http.Request) {
	snapshot := a.Blacklist.Snapshot()
	type banEntry struct {
		IP        string `json:"ip"`
		ExpiresAt string `json:"expires_at,omitempty"` // vazio = permanente
	}
	out := make([]banEntry, 0, len(snapshot))
	for ip, expiresAt := range snapshot {
		e := banEntry{IP: ip}
		if !expiresAt.IsZero() {
			e.ExpiresAt = expiresAt.Format(time.RFC3339)
		}
		out = append(out, e)
	}
	writeJSON(w, http.StatusOK, out)
}

type banRequest struct {
	IP              string `json:"ip"`
	DurationSeconds int    `json:"duration_seconds"` // 0 = permanente
}

func (a *API) handleBan(w http.ResponseWriter, r *http.Request) {
	var req banRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.IP == "" {
		http.Error(w, `{"error":"payload inválido, esperado {ip, duration_seconds}"}`, http.StatusBadRequest)
		return
	}
	a.Blacklist.Ban(req.IP, time.Duration(req.DurationSeconds)*time.Second)
	writeJSON(w, http.StatusOK, map[string]string{"status": "banido", "ip": req.IP})
}

func (a *API) handleUnban(w http.ResponseWriter, r *http.Request) {
	ip := r.PathValue("ip")
	a.Blacklist.Unban(ip)
	writeJSON(w, http.StatusOK, map[string]string{"status": "desbloqueado", "ip": ip})
}

// --- routing rules / portas -------------------------------------------

func (a *API) handleListRules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.Router.List())
}

func (a *API) handleUpsertRule(w http.ResponseWriter, r *http.Request) {
	var rule models.RoutingRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, `{"error":"payload inválido"}`, http.StatusBadRequest)
		return
	}
	if rule.Subdomain == "" || rule.PathPattern == "" || rule.DestinationAddr == "" {
		http.Error(w, `{"error":"subdomain, path_pattern e destination_addr são obrigatórios"}`, http.StatusBadRequest)
		return
	}
	if rule.ID == "" {
		rule.ID = randomID()
	}
	if !rule.Enabled {
		rule.Enabled = true // default: regra criada já entra ativa
	}
	a.Router.Upsert(rule)
	writeJSON(w, http.StatusOK, rule)
}

func (a *API) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.Router.Delete(id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "removida", "id": id})
}

// --- helpers -----------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func randomID() string {
	return "rule-" + idgen.New()
}
