package gateway

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// newReverseProxy cria um httputil.ReverseProxy apontando para um destino
// dinâmico, com timeout de upstream e injeção do correlation ID como
// header interno (rastreabilidade ponta a ponta).
//
// Autenticação de serviço (mTLS ou token interno assinado) deve ser
// configurada no Transport abaixo em produção — aqui deixamos o ponto
// de extensão marcado.
func newReverseProxy(destAddr string, upstreamTimeout time.Duration) *httputil.ReverseProxy {
	target := &url.URL{Scheme: "http", Host: destAddr}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(req *httputil.ProxyRequest) {
			// DEFINE O TARGET
			req.Out.URL.Scheme = target.Scheme
			req.Out.URL.Host = target.Host
			req.Out.URL.Path = req.In.URL.Path
			req.Out.URL.RawQuery = req.In.URL.RawQuery

			// HEADERS
			req.Out.Header.Set("X-Forwarded-Host", req.In.Host)
			req.Out.Header.Set("X-Forwarded-Proto", "https")

			if cid, ok := req.In.Context().Value(ctxKeyCorrelationID).(string); ok {
				req.Out.Header.Set("X-Correlation-ID", cid)
			}
		},
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: upstreamTimeout,
			}).DialContext,
			ResponseHeaderTimeout: upstreamTimeout,
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			println("❌ PROXY ERROR:", err.Error())
			w.WriteHeader(http.StatusBadGateway)
		},
	}

	return proxy
}

type ctxKey int

const ctxKeyCorrelationID ctxKey = iota

func withCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyCorrelationID, id)
}
