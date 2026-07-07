package gateway

import (
	"context"
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
	target, _ := url.Parse("http://" + destAddr)
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Sobrescreve o Director padrão para corrigir o Host header
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Passa o host do destino, não o host original do cliente
		// Sem isso o Next.js recebe "workon.merchonline.com.br" e quebra
		req.Host = target.Host
	}

	proxy.Transport = &http.Transport{
		ResponseHeaderTimeout: upstreamTimeout,
	}

	return proxy
	/*
		target := &url.URL{Scheme: "http", Host: destAddr}

		proxy := httputil.NewSingleHostReverseProxy(target)

		transport := &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: upstreamTimeout,
			}).DialContext,
			ResponseHeaderTimeout: upstreamTimeout,
			// TLSClientConfig: configurar mTLS aqui em produção (client cert
			// assinado pela CA interna, para autenticar o Core perante o
			// serviço interno).
		}
		proxy.Transport = transport

		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			// Nunca confiar em headers vindos do cliente externo — o Nginx já
			// deveria ter limpo, mas o Core reforça isso aqui também.
			req.Header.Del("X-Forwarded-For")
			req.Header.Del("X-Real-IP")

			if cid, ok := req.Context().Value(ctxKeyCorrelationID).(string); ok {
				req.Header.Set("X-Correlation-ID", cid)
			}
			// TODO produção: injetar token interno assinado / client cert:
			// req.Header.Set("X-Internal-Auth", signedToken)
		}

		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			w.WriteHeader(http.StatusBadGateway)
		}

		return proxy
	*/
}

type ctxKey int

const ctxKeyCorrelationID ctxKey = iota

func withCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyCorrelationID, id)
}
