// placeholder http_proxy.go
package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"

	"github.com/elazarl/goproxy"
	"go.uber.org/zap"

	"github.com/lianshufeng/proxy-pool/internal/config"
	"github.com/lianshufeng/proxy-pool/internal/pool"
	"github.com/lianshufeng/proxy-pool/internal/upstream"
)

type Server struct{ *http.Server }

func Start(ctx context.Context, log *zap.Logger, cfg config.Config, pp *pool.Pool) *Server {
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = false

	// 普通 HTTP：每次请求由 Proxy 回调选择上游
	tr := &http.Transport{
		Proxy: func(*http.Request) (*url.URL, error) {
			addr, ok := pp.Get()
			if !ok {
				return nil, errors.New("no upstream")
			}
			return url.Parse(ensureScheme(addr))
		},
		IdleConnTimeout:       cfg.IdleTimeout,
		ResponseHeaderTimeout: cfg.HeaderTimeout,
		TLSHandshakeTimeout:   cfg.DialTimeout,
	}
	proxy.Tr = tr

	// HTTPS：覆盖 CONNECT 的拨号，通过“二次 CONNECT”
	proxy.ConnectDial = func(network, addr string) (net.Conn, error) {
		c, cancel := context.WithTimeout(ctx, cfg.DialTimeout)
		defer cancel()
		up, ok := pp.Get()
		if !ok {
			return nil, errors.New("no upstream")
		}
		return upstream.DialViaHTTPProxy(c, up, addr, cfg.DialTimeout)
	}

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      proxy,
		ReadTimeout:  cfg.DialTimeout,
		WriteTimeout: cfg.IdleTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("proxy server error", zap.Error(err))
		}
	}()
	return &Server{srv}
}

func ensureScheme(s string) string {
	if len(s) >= 7 && (s[:7] == "http://" || (len(s) >= 8 && s[:8] == "https://")) {
		return s
	}
	return "http://" + s
}

func (s *Server) Shutdown(ctx context.Context) error { return s.Server.Shutdown(ctx) }
