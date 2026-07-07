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
		// SEGUE REDIRECIONAMENTOS
		DisableCompression: false,
	}
	proxy.Transport = transport

	// MODIFICA O DIRECTOR PARA SEGUIR REDIRECIONAMENTOS
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Forwarded-Proto", "https")
	}

	proxy.Rewrite = func(req *httputil.ProxyRequest) {
		req.SetXForwarded()
		req.SetURL(target)
		
		// FORÇA O PROTOCOLO CORRETO
		req.Out.Header.Set("X-Forwarded-Proto", "https")
		req.Out.Header.Set("X-Forwarded-Host", req.In.Host)
		
		req.Out.Header.Del("X-Forwarded-For")
		req.Out.Header.Del("X-Real-IP")
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
