// internal/metrics/metrics.go
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Server 包装 http.Server 便于优雅关闭
type Server struct{ *http.Server }

// Start 启动一个仅暴露 /metrics 的 HTTP 服务。
// 如果 addr 为空字符串则不会启动服务，直接返回 nil。
func Start(log *zap.Logger, addr string) *Server {
	if addr == "" {
		return nil
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// 异步启动
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Warn("metrics server error", zap.Error(err))
		}
	}()

	return &Server{srv}
}
