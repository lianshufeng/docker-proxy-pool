// placeholder main.go - see conversation for full code
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lianshufeng/proxy-pool/internal/config"
	"github.com/lianshufeng/proxy-pool/internal/fetcher"
	"github.com/lianshufeng/proxy-pool/internal/log"
	"github.com/lianshufeng/proxy-pool/internal/metrics"
	"github.com/lianshufeng/proxy-pool/internal/pool"
	"github.com/lianshufeng/proxy-pool/internal/server"
)

var (
	version = "dev" // 由 ldflags 注入
	commit  = "none"
	date    = "unknown"
)

func main() {
	cfg := config.Parse()
	logger := log.New(cfg.Env)
	defer logger.Sync()

	logger.Infow("starting",
		"version", version, "commit", commit, "date", date,
		"listen", cfg.ListenAddr, "api", cfg.APIURL,
		"interval", cfg.FetchInterval.String(), "ttl", cfg.TTL.String(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 代理池 & 拉取任务
	pp := pool.New()
	fetcher.Start(ctx, logger.Desugar(), cfg, pp)

	// 启动 Proxy 与 Metrics
	proxySrv := server.Start(ctx, logger.Desugar(), cfg, pp)
	metricSrv := metrics.Start(logger.Desugar(), cfg.MetricsListen)

	// 优雅退出
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Infow("shutting down...")

	shutdown := func(name string, fn func(context.Context) error, d time.Duration) {
		c, cancel := context.WithTimeout(context.Background(), d)
		defer cancel()
		_ = fn(c)
		logger.Infow(fmt.Sprintf("%s stopped", name))
	}
	shutdown("proxy", proxySrv.Shutdown, 5*time.Second)
	if metricSrv != nil {
		shutdown("metrics", metricSrv.Shutdown, 3*time.Second)
	}
	logger.Infow("bye")
}
