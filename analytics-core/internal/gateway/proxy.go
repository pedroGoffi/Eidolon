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

	proxy := httputil.NewSingleHostReverseProxy(target)

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: upstreamTimeout,
		}).DialContext,
		ResponseHeaderTimeout: upstreamTimeout,
		DisableCompression:    false,
	}
	proxy.Transport = transport

	// USA SÓ REWRITE - REMOVE O DIRECTOR
	proxy.Rewrite = func(req *httputil.ProxyRequest) {
		// Define o target
		req.SetURL(target)

		// Headers de forward
		req.Out.Header.Set("X-Forwarded-Host", req.In.Host)
		req.Out.Header.Set("X-Forwarded-Proto", "https")
		req.Out.Header.Set("X-Forwarded-For", req.In.RemoteAddr)

		// Remove headers maliciosos
		req.Out.Header.Del("X-Forwarded-For")
		req.Out.Header.Del("X-Real-IP")

		// Correlation ID
		if cid, ok := req.In.Context().Value(ctxKeyCorrelationID).(string); ok {
			req.Out.Header.Set("X-Correlation-ID", cid)
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		println("❌ PROXY ERROR:", err.Error())
		w.WriteHeader(http.StatusBadGateway)
	}

	return proxy
}

type ctxKey int

const ctxKeyCorrelationID ctxKey = iota

func withCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyCorrelationID, id)
}
