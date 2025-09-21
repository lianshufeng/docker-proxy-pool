// placeholder fetcher.go
package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/lianshufeng/proxy-pool/internal/config"
	"github.com/lianshufeng/proxy-pool/internal/pool"
)

func Start(ctx context.Context, log *zap.Logger, cfg config.Config, p *pool.Pool) {
	// 立即拉一次
	go func() {
		if list, err := fetch(cfg.APIURL); err != nil {
			log.Warn("initial fetch failed", zap.Error(err))
		} else {
			p.Set(list, cfg.TTL)
		}
		p.Clean()
	}()

	ticker := time.NewTicker(cfg.FetchInterval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				list, err := fetch(cfg.APIURL)
				if err != nil {
					log.Warn("fetch failed", zap.Error(err))
				} else if len(list) > 0 {
					p.Set(list, cfg.TTL)
					log.Info("proxies refreshed", zap.Int("count", len(list)))
				}
				p.Clean()
			}
		}
	}()
}

func fetch(api string) ([]string, error) {
	cli := &http.Client{Timeout: 10 * time.Second}
	resp, err := cli.Get(api)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// JSON 数组
	var arr []string
	if json.Unmarshal(body, &arr) == nil {
		return arr, nil
	}
	// 换行文本
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if s := strings.TrimSpace(ln); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no proxies parsed")
	}
	return out, nil
}
