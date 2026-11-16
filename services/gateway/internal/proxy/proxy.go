// Package proxy implementa o roteamento e as políticas do API Gateway.
package proxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/davidrodma/innkeep/pkg/kernel"
	"github.com/davidrodma/innkeep/pkg/kernel/access"
	"github.com/davidrodma/innkeep/pkg/platform/auth"
	"github.com/davidrodma/innkeep/pkg/platform/httpx"
	"github.com/davidrodma/innkeep/pkg/platform/redisx"
)

// Route associa um prefixo de caminho a um serviço.
type Route struct {
	Prefix   string
	Upstream string
	// Public: a rota não exige token (site público, login).
	Public bool
	// Sensitive: rota pública com limite de requisições mais rígido (login).
	Sensitive bool
}

// Limits são os tetos de requisições por janela.
type Limits struct {
	Public        int
	Sensitive     int
	Authenticated int
	Window        time.Duration
}

// RevocationChecker informa se um token de acesso foi revogado (logout).
type RevocationChecker interface {
	IsRevoked(ctx context.Context, tokenID string) (bool, error)
}

// RateLimiter decide se uma requisição cabe no limite.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (redisx.Decision, error)
}

// Options configura o gateway.
type Options struct {
	Routes         []Route
	Verifier       auth.TokenVerifier
	Revocations    RevocationChecker
	Limiter        RateLimiter
	Limits         Limits
	TrustedProxies int
	Logger         *slog.Logger
}

type target struct {
	route Route
	proxy *httputil.ReverseProxy
}

// Gateway é o handler HTTP que aplica as políticas e encaminha a requisição.
type Gateway struct {
	targets []target
	options Options
}

// New monta o gateway, criando um proxy reverso por serviço.
func New(options Options) (*Gateway, error) {
	gateway := &Gateway{options: options}
	// Transporte compartilhado: reaproveita conexões com os serviços e
	// propaga o contexto de tracing (cabeçalho traceparent) em cada chamada.
	transport := otelhttp.NewTransport(&http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	})

	for _, route := range options.Routes {
		upstream, err := url.Parse(route.Upstream)
		if err != nil || upstream.Host == "" {
			return nil, fmt.Errorf("gateway: endereço inválido para %s: %q", route.Prefix, route.Upstream)
		}
		reverse := &httputil.ReverseProxy{
			Transport: transport,
			Rewrite: func(r *httputil.ProxyRequest) {
				r.SetURL(upstream)
				r.Out.Host = upstream.Host
				// O gateway é a borda: descarta o que o cliente mandou em
				// X-Forwarded-* e escreve os valores reais.
				r.SetXForwarded()
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				if errors.Is(err, context.Canceled) {
					return // o cliente desistiu da requisição
				}
				options.Logger.ErrorContext(r.Context(), "serviço indisponível",
					slog.String("upstream", upstream.Host), slog.Any("erro", err))
				httpx.Error(w, r, kernel.Unavailable("gateway.upstream_unavailable", "serviço temporariamente indisponível"))
			},
		}
		gateway.targets = append(gateway.targets, target{route: route, proxy: reverse})
	}
	return gateway, nil
}

// ServeHTTP aplica, nesta ordem: roteamento, autenticação, revogação, limite
// de requisições — e então encaminha ao serviço.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	matched := g.match(r.URL.Path)
	if matched == nil {
		httpx.Error(w, r, kernel.NotFound("gateway.route_not_found", "rota inexistente"))
		return
	}
	ctx := r.Context()
	clientIP := g.clientIP(r)

	limitKey, limit := "ip:"+clientIP, g.options.Limits.Public
	if matched.route.Sensitive {
		limitKey, limit = "auth:"+clientIP, g.options.Limits.Sensitive
	}

	if !matched.route.Public {
		principal, err := g.authenticate(r)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		limitKey, limit = "user:"+principal.UserID, g.options.Limits.Authenticated
	}

	decision, err := g.options.Limiter.Allow(ctx, limitKey, limit, g.options.Limits.Window)
	if err != nil {
		// "Fail open": se o Redis cair, a API continua atendendo sem o
		// limite. Derrubar o sistema inteiro por causa do limitador seria
		// pior do que ficar alguns minutos sem ele.
		g.options.Logger.WarnContext(ctx, "limitador de requisições indisponível", slog.Any("erro", err))
	} else {
		for name, value := range decision.Headers() {
			w.Header().Set(name, value)
		}
		if !decision.Allowed {
			httpx.Error(w, r, &kernel.Error{
				Kind: kernel.KindRateLimited, Code: "gateway.rate_limited",
				Message: "muitas requisições; aguarde um instante e tente novamente",
			})
			return
		}
	}

	matched.proxy.ServeHTTP(w, r)
}

// authenticate valida o token antes de incomodar qualquer serviço. Os
// serviços validam de novo (defesa em profundidade); aqui o ganho é barrar
// cedo o tráfego não autenticado e aplicar a revogação imediata do logout.
func (g *Gateway) authenticate(r *http.Request) (access.Principal, error) {
	token := auth.BearerToken(r)
	if token == "" {
		return access.Principal{}, kernel.Unauthorized("auth.missing_token", "autenticação necessária")
	}
	principal, err := g.options.Verifier.Verify(r.Context(), token)
	if err != nil {
		return access.Principal{}, err
	}
	revoked, err := g.options.Revocations.IsRevoked(r.Context(), principal.TokenID)
	if err != nil {
		g.options.Logger.WarnContext(r.Context(), "lista de tokens revogados indisponível", slog.Any("erro", err))
	}
	if revoked {
		return access.Principal{}, kernel.Unauthorized("auth.token_revoked", "sessão encerrada; faça login novamente")
	}
	return principal, nil
}

func (g *Gateway) match(path string) *target {
	for i := range g.targets {
		prefix := g.targets[i].route.Prefix
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return &g.targets[i]
		}
	}
	return nil
}

// clientIP descobre o IP real do cliente.
//
// X-Forwarded-For é uma lista "cliente, proxy1, proxy2" — e o cliente pode
// escrever o que quiser nela. Só são confiáveis as entradas acrescentadas
// pelos NOSSOS proxies, que ficam no fim da lista. Por isso contamos a partir
// do fim, pulando o número configurado de proxies confiáveis. Usar a primeira
// entrada às cegas permitiria burlar o limite de requisições trocando o
// cabeçalho a cada chamada.
func (g *Gateway) clientIP(r *http.Request) string {
	remote, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remote = r.RemoteAddr
	}
	if g.options.TrustedProxies <= 0 {
		return remote
	}
	var hops []string
	for _, header := range r.Header.Values("X-Forwarded-For") {
		for _, hop := range strings.Split(header, ",") {
			if hop = strings.TrimSpace(hop); hop != "" {
				hops = append(hops, hop)
			}
		}
	}
	index := len(hops) - g.options.TrustedProxies
	if index < 0 || index >= len(hops) {
		return remote
	}
	return hops[index]
}
