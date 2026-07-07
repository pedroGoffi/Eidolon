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
	}
	proxy.Transport = transport

	proxy.Rewrite = func(req *httputil.ProxyRequest) {
		// Mantém o comportamento padrão do NewSingleHostReverseProxy
		req.SetXForwarded()
		req.SetURL(target)

		// Nunca confiar em headers vindos do cliente externo
		req.Out.Header.Del("X-Forwarded-For")
		req.Out.Header.Del("X-Real-IP")

		if cid, ok := req.In.Context().Value(ctxKeyCorrelationID).(string); ok {
			req.Out.Header.Set("X-Correlation-ID", cid)
		}
		// TODO produção: injetar token interno assinado
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusBadGateway)
	}

	return proxy
}

type ctxKey int

const ctxKeyCorrelationID ctxKey = iota

func withCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyCorrelationID, id)
}
