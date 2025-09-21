// placeholder config.go
package config

import "time"
import "flag"

type Config struct {
	Env           string // "prod" | "dev"
	ListenAddr    string
	APIURL        string
	FetchInterval time.Duration
	TTL           time.Duration
	DialTimeout   time.Duration
	HeaderTimeout time.Duration
	IdleTimeout   time.Duration
	MetricsListen string
}

func Parse() Config {
	var c Config
	flag.StringVar(&c.Env, "env", "prod", "env: prod|dev")
	flag.StringVar(&c.ListenAddr, "listen", ":6808", "proxy listen addr")
	flag.StringVar(&c.APIURL, "api-url", "", "proxy list API (JSON array or newline-separated)")
	flag.DurationVar(&c.FetchInterval, "fetch-interval", 60*time.Second, "fetch interval")
	flag.DurationVar(&c.TTL, "ttl", 2*time.Minute, "per-batch TTL")
	flag.DurationVar(&c.DialTimeout, "dial-timeout", 10*time.Second, "dial/TLS timeout")
	flag.DurationVar(&c.HeaderTimeout, "resp-header-timeout", 20*time.Second, "upstream header timeout")
	flag.DurationVar(&c.IdleTimeout, "idle-timeout", 90*time.Second, "idle conn timeout")
	flag.StringVar(&c.MetricsListen, "metrics-listen", ":2112", "metrics listen addr (empty to disable)")
	flag.Parse()
	if c.APIURL == "" {
		panic("--api-url is required")
	}
	return c
}
