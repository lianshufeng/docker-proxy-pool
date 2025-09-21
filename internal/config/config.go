package config

import (
	"flag"
	"time"
)

type Config struct {
	Listen         string        // 代理对外监听地址，例 :6808
	APIURL         string        // 上游代理列表 API（必填）
	FetchInterval  time.Duration // （保留旧参数）批量拉取间隔，若不用可忽略
	AppendInterval time.Duration // 新增：每隔该时间追加 1 个代理到池子
	TTL            time.Duration // 每个代理的生存时长
	MetricsListen  string        // Prometheus /metrics 监听地址（留空则关闭）

	// 连接/超时配置
	DialTimeout      time.Duration
	IdleConn         int
	IdleTimeout      time.Duration
	HandshakeTimeout time.Duration
}

func Parse() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.Listen, "listen", ":6808", "代理对外监听地址，例 :6808")
	flag.StringVar(&cfg.APIURL, "api-url", "", "上游代理列表 API（必填）")
	flag.DurationVar(&cfg.FetchInterval, "fetch-interval", 60*time.Second, "（保留旧参数）批量拉取间隔，若不用可忽略")
	flag.DurationVar(&cfg.AppendInterval, "append-interval", 10*time.Second, "每隔该时间从 API 追加 1 个代理到池子")
	flag.DurationVar(&cfg.TTL, "ttl", 2*time.Minute, "每个代理的生存时长")
	flag.StringVar(&cfg.MetricsListen, "metrics-listen", ":2112", "Prometheus /metrics 监听地址（留空则关闭）")

	flag.DurationVar(&cfg.DialTimeout, "dial-timeout", 10*time.Second, "拨号超时时间")
	flag.IntVar(&cfg.IdleConn, "idle-conns", 100, "传输最大空闲连接数")
	flag.DurationVar(&cfg.IdleTimeout, "idle-timeout", 90*time.Second, "传输空闲超时时间")
	flag.DurationVar(&cfg.HandshakeTimeout, "handshake-timeout", 10*time.Second, "TLS 握手超时时间")

	flag.Parse()
	return cfg
}
