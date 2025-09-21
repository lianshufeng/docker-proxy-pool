package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lianshufeng/proxy-pool/internal/config"
	"github.com/lianshufeng/proxy-pool/internal/fetcher"
	"github.com/lianshufeng/proxy-pool/internal/pool"
	"github.com/lianshufeng/proxy-pool/internal/server"
)

func main() {
	// —— 强制把日志打到标准输出，并带微秒+文件行号 —— //
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)

	log.Println("[BOOT] ===== proxy-pool starting (this banner proves you are running the new binary) =====")

	cfg := config.Parse()
	if cfg.APIURL == "" {
		log.Fatal("missing --api-url")
	}

	pl := pool.New()
	ft := fetcher.New(cfg.APIURL, cfg.DialTimeout)

	srv := server.New(server.Options{
		Listen:              cfg.Listen,
		Pool:                pl,
		DialTimeout:         cfg.DialTimeout,
		IdleConns:           cfg.IdleConn,
		IdleTimeout:         cfg.IdleTimeout,
		TLSHandshakeTimeout: cfg.HandshakeTimeout,
	})

	log.Printf("[BOOT] listen=%s api-url=%s append-interval=%s ttl=%s", cfg.Listen, cfg.APIURL, cfg.AppendInterval, cfg.TTL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 每隔 append-interval 追加 1 个代理
	go func() {
		iv := cfg.AppendInterval
		if iv <= 0 {
			iv = 10 * time.Second
		}
		tk := time.NewTicker(iv)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				addr, err := ft.Next(ctx)
				if err != nil {
					log.Printf("[APPEND] fetch next failed: %v", err)
					continue
				}
				pl.Add(addr, cfg.TTL)
				log.Printf("[APPEND] added=%q size=%d", addr, pl.Size())
			case <-ctx.Done():
				return
			}
		}
	}()

	// 定期清理过期项
	go func() {
		tk := time.NewTicker(30 * time.Second)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				before := pl.Size()
				pl.Sweep()
				after := pl.Size()
				log.Printf("[SWEEP] before=%d after=%d", before, after)
			case <-ctx.Done():
				return
			}
		}
	}()

	// 启动代理
	go func() {
		log.Printf("[BOOT] starting proxy server on %s ...", cfg.Listen)
		if err := srv.Start(); err != nil {
			log.Printf("[EXIT] proxy server stopped: %v", err)
		}
	}()

	// 优雅退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Printf("[EXIT] shutting down...")
	cancel()
	_ = srv.Shutdown()
}
