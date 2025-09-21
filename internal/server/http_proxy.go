package server

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/lianshufeng/proxy-pool/internal/pool"
)

type Server struct {
	httpSrv *http.Server
	proxy   *goproxy.ProxyHttpServer
	opts    Options
}

type Options struct {
	Listen              string
	Pool                *pool.Pool
	DialTimeout         time.Duration
	IdleConns           int
	IdleTimeout         time.Duration
	TLSHandshakeTimeout time.Duration
}

// 兼容解析：支持 http:// 以及无 scheme 的 "user:pass@host:port" / "host:port"
// 现在仅考虑 HTTP 代理：传入 https:// 或 socks5:// 会被判定为不支持并从池中删除。
func parseUpstream(addr string) (u *url.URL, hasScheme bool, err error) {
	if strings.Contains(addr, "://") {
		u, err = url.Parse(addr)
		return u, true, err
	}
	u, err = url.Parse("//" + addr) // 把它当 authority 解析
	return u, false, err
}

// 动态删除上游：通过多种可能的方法名做类型断言，尽量兼容你的 pool 实现。
// 如果没有对应方法，会打印日志但不 panic。
func removeFromPool(p *pool.Pool, addr string) {
	type remover interface{ Remove(string) }
	type deleter interface{ Delete(string) }
	type del interface{ Del(string) }
	type ban interface{ Ban(string) }
	type blacklist interface{ Blacklist(string) }
	type invalidate interface{ Invalidate(string) }
	type markbad interface{ MarkBad(string) }
	type fail interface{ Fail(string) }
	type drop interface{ Drop(string) }

	if addr == "" {
		return
	}
	if v, ok := any(p).(remover); ok {
		v.Remove(addr)
		return
	}
	if v, ok := any(p).(deleter); ok {
		v.Delete(addr)
		return
	}
	if v, ok := any(p).(del); ok {
		v.Del(addr)
		return
	}
	if v, ok := any(p).(ban); ok {
		v.Ban(addr)
		return
	}
	if v, ok := any(p).(blacklist); ok {
		v.Blacklist(addr)
		return
	}
	if v, ok := any(p).(invalidate); ok {
		v.Invalidate(addr)
		return
	}
	if v, ok := any(p).(markbad); ok {
		v.MarkBad(addr)
		return
	}
	if v, ok := any(p).(fail); ok {
		v.Fail(addr)
		return
	}
	if v, ok := any(p).(drop); ok {
		v.Drop(addr)
		return
	}
	log.Printf("[POOL] cannot remove %q: pool doesn't expose a remove method", addr)
}

func New(opts Options) *Server {
	prx := goproxy.NewProxyHttpServer()

	// 打开 goproxy 的日志
	prx.Verbose = true
	prx.Logger = log.New(os.Stdout, "[GOPROXY] ", log.LstdFlags|log.Lmicroseconds)

	// ---------- 普通 HTTP：每请求动态选上游（仅 http） ----------
	tr := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			if addr, ok := opts.Pool.Get(); ok {
				u, hasScheme, err := parseUpstream(addr)
				if err != nil || u.Host == "" {
					log.Printf("[HTTP] upstream parse error: %v (addr=%q) -> remove & direct", err, addr)
					removeFromPool(opts.Pool, addr)
					return nil, nil // 走直连
				}
				// 只允许 http 代理
				if !hasScheme || u.Scheme == "" {
					u.Scheme = "http"
				}
				if strings.ToLower(u.Scheme) != "http" {
					log.Printf("[HTTP] upstream scheme %q not supported; remove & direct (addr=%q)", u.Scheme, addr)
					removeFromPool(opts.Pool, addr)
					return nil, nil
				}
				log.Printf("[HTTP] %s %s upstream=%q parsed=%s|%s", req.Method, req.URL.String(), addr, u.Scheme, u.Host)
				return u, nil
			}
			log.Printf("[HTTP] no upstream -> direct %s %s", req.Method, req.URL.String())
			return nil, nil
		},
		DialContext: (&net.Dialer{
			Timeout:   opts.DialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        opts.IdleConns,
		IdleConnTimeout:     opts.IdleTimeout,
		TLSHandshakeTimeout: opts.TLSHandshakeTimeout,
		ForceAttemptHTTP2:   true,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: false},
	}
	prx.Tr = tr

	// ---------- HTTPS CONNECT：仅支持 http 上游代理 ----------
	prx.ConnectDial = func(network, targetAddr string) (net.Conn, error) {
		upstream, ok := opts.Pool.Get()
		if !ok || strings.TrimSpace(upstream) == "" {
			log.Printf("[CONNECT] no upstream -> direct dial %s %s", network, targetAddr)
			d := net.Dialer{Timeout: opts.DialTimeout}
			return d.Dial(network, targetAddr)
		}
		u, _, err := parseUpstream(upstream)
		if err != nil || u.Host == "" {
			log.Printf("[CONNECT] upstream parse error: %v (addr=%q) -> remove & direct", err, upstream)
			removeFromPool(opts.Pool, upstream)
			d := net.Dialer{Timeout: opts.DialTimeout}
			return d.Dial(network, targetAddr)
		}
		// 仅允许 http；无 scheme 也按 http 处理
		if u.Scheme == "" {
			u.Scheme = "http"
		}
		if s := strings.ToLower(u.Scheme); s != "http" {
			log.Printf("[CONNECT] unsupported upstream scheme=%q; remove & direct (addr=%q)", u.Scheme, upstream)
			removeFromPool(opts.Pool, upstream)
			d := net.Dialer{Timeout: opts.DialTimeout}
			return d.Dial(network, targetAddr)
		}

		host := u.Host
		if !strings.Contains(host, ":") {
			host += ":80"
		}
		log.Printf("[CONNECT] try HTTP-proxy (only) upstreamHost=%s target=%s", host, targetAddr)

		raw, err := net.DialTimeout("tcp", host, opts.DialTimeout)
		if err != nil {
			log.Printf("[CONNECT] dial upstream failed: %v -> remove & direct", err)
			removeFromPool(opts.Pool, upstream)
			d := net.Dialer{Timeout: opts.DialTimeout}
			return d.Dial(network, targetAddr)
		}
		conn := net.Conn(raw)

		// 写+读 12s 超时，避免卡死
		_ = conn.SetDeadline(time.Now().Add(12 * time.Second))

		req := &http.Request{
			Method: "CONNECT",
			URL:    &url.URL{Opaque: targetAddr},
			Host:   targetAddr,
			Header: make(http.Header),
		}
		if u.User != nil {
			user := u.User.Username()
			pass, _ := u.User.Password()
			token := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
			req.Header.Set("Proxy-Authorization", "Basic "+token)
			log.Printf("[CONNECT] using basic auth for upstream user=%q", user)
		}
		req.Header.Set("Proxy-Connection", "Keep-Alive")

		log.Printf("[CONNECT] write CONNECT to upstream")
		if err := req.Write(conn); err != nil {
			_ = conn.Close()
			log.Printf("[CONNECT] write CONNECT fail: %v -> remove & direct", err)
			removeFromPool(opts.Pool, upstream)
			d := net.Dialer{Timeout: opts.DialTimeout}
			return d.Dial(network, targetAddr)
		}

		br := bufio.NewReader(conn)
		resp, err := http.ReadResponse(br, req)
		if err != nil {
			_ = conn.Close()
			log.Printf("[CONNECT] read upstream response fail: %v -> remove & direct", err)
			removeFromPool(opts.Pool, upstream)
			d := net.Dialer{Timeout: opts.DialTimeout}
			return d.Dial(network, targetAddr)
		}
		defer func() {
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
		}()

		if resp.StatusCode != http.StatusOK {
			_ = conn.Close()
			log.Printf("[CONNECT] upstream CONNECT failed: %s -> remove & direct", resp.Status)
			removeFromPool(opts.Pool, upstream)
			d := net.Dialer{Timeout: opts.DialTimeout}
			return d.Dial(network, targetAddr)
		}

		// 隧道建立成功后清理 deadline，交给后续长连接
		_ = conn.SetDeadline(time.Time{})
		log.Printf("[CONNECT] tunnel established via HTTP-proxy")
		return conn, nil
	}

	s := &Server{
		proxy: prx,
		opts:  opts,
	}

	s.httpSrv = &http.Server{
		Addr:    opts.Listen,
		Handler: logMiddleware(prx),
	}

	return s
}

// --- 简单连接日志中间件（可选） ---

type loggingListener struct {
	net.Listener
}

func (l loggingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err == nil {
		log.Printf("[ACCEPT] from=%s -> local=%s", c.RemoteAddr(), c.LocalAddr())
	}
	return c, err
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			log.Printf("[IN] CONNECT Host=%s URL=%q From=%s", r.Host, r.URL.String(), r.RemoteAddr)
		} else {
			log.Printf("[IN] %s %s Host=%s From=%s", r.Method, r.URL.String(), r.Host, r.RemoteAddr)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.opts.Listen)
	if err != nil {
		log.Printf("[START] listen error: %v", err)
		return err
	}
	log.Printf("[START] listening on %s", s.opts.Listen)
	return s.httpSrv.Serve(loggingListener{ln})
}

func (s *Server) Shutdown() error {
	return s.httpSrv.Close()
}
