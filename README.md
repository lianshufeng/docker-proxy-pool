# proxy-pool

Version: `0.1.0`

A lightweight HTTP/HTTPS forward proxy with a **rotating upstream pool** and optional Prometheus metrics.


```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant P as proxy-pool
    participant U as HTTP Upstream
    participant T as Target

    C->>P: HTTP/HTTPS request
    P->>U: Forward (or CONNECT)
    U->>T: Relay to target
    T-->>U: Response
    U-->>P: Response
    P-->>C: Response

    note over P: Round‑robin pick from pool, TTL & auto‑remove bad upstreams
```


## Features
- HTTP/HTTPS forward proxy with a **rotating proxy pool** (round‑robin).
- **Pulls upstream proxies** from an API endpoint, supports JSON array or newline‑separated text.
- Per‑proxy **TTL**; expired entries are evicted automatically.
- For each request, dynamically picks one upstream; auto‑removes bad/unsupported entries from the pool.
- **HTTP upstream only** (supports `http://user:pass@host:port` or `host:port`, `user:pass@host:port`). SOCKS is not supported.
- **HTTPS tunneling** via `CONNECT` through HTTP upstream; if the chosen upstream fails, it **falls back to direct**.
- Optional **Prometheus /metrics** server.
- Dockerfile and simple build scripts for Linux/Windows.

## Quick Start

```bash
docker run -d --name proxy-pool \
  -p 6808:6808 -p 2112:2112 \
  lianshufeng/proxy-pool:latest \
  --api-url="http://YOUR_API_ENDPOINT" \
  --listen=":6808" \
  --append-interval="10s" \
  --ttl="2m" \
  --metrics-listen=":2112"
```

Test:

```bash
# HTTP request through the proxy
curl -x http://127.0.0.1:6808 http://httpbin.org/ip

# HTTPS request via CONNECT
curl -x http://127.0.0.1:6808 https://www.example.com -I
```

## Configuration

| Flag | Default | Description |
|---|---:|---|
| `--listen` | `:6808` | Address for the proxy server (e.g., `:6808`). |
| `--api-url` | **required** | Endpoint returning upstream proxies (JSON array or newline text). |
| `--append-interval` | `10s` | Interval to append **one** proxy from API into the pool. |
| `--fetch-interval` | `60s` | (Legacy) batch fetch interval; can be ignored if not used. |
| `--ttl` | `2m` | Time to live for each proxy before it expires. |
| `--metrics-listen` | `:2112` | Prometheus server for `/metrics` (empty to disable). |
| `--dial-timeout` | `10s` | Dial timeout. |
| `--idle-conns` | `100` | Max idle connections for transport. |
| `--idle-timeout` | `90s` | Idle timeout for transport. |
| `--handshake-timeout` | `10s` | TLS handshake timeout. |


### Upstream API format

API may return either:

- **JSON array**
  ```json
  ["http://user:pass@10.0.0.1:8080", "10.0.0.2:8080", "user:pass@10.0.0.3:8080"]
  ```

- **Newline‑separated text**
  ```text
  http://user:pass@10.0.0.1:8080
  10.0.0.2:8080
  user:pass@10.0.0.3:8080
  ```


## Docker Compose (example)
```yaml
services:
  proxy-pool:
    image: lianshufeng/proxy-pool:latest
    ports:
      - "6808:6808"          # proxy
      - "2112:2112"          # metrics (optional)
    command: >
      --api-url="http://YOUR_API_ENDPOINT"
      --listen=":6808"
      --append-interval="10s"
      --ttl="2m"
      --metrics-listen=":2112"
    restart: always
```

## Build

### Build (native)

```bash
go mod tidy
go build -o proxy-pool ./cmd/proxy-pool
./proxy-pool --api-url="http://YOUR_API_ENDPOINT"
```

### Build (Docker)

```bash
docker build -t lianshufeng/proxy-pool:latest .
docker run -d --name proxy-pool \
  -p 6808:6808 -p 2112:2112 \
  lianshufeng/proxy-pool:latest \
  --api-url="http://YOUR_API_ENDPOINT" \
  --listen=":6808" \
  --append-interval="10s" \
  --ttl="2m" \
  --metrics-listen=":2112"
```


## Metrics
If `--metrics-listen` is set (default `:2112`), a small HTTP server exposes Prometheus metrics at `/metrics`.

## Notes & Limitations

- This is a **forward proxy** without built‑in client authentication or ACL. **Do not expose it to the public internet**. Bind to a private interface or protect with firewall.
- Upstream **HTTP** proxies only. SOCKS/`https://` upstream entries are detected and removed.
- For HTTPS, the proxy sends `CONNECT` to the chosen HTTP upstream. If it fails, the server may **fallback to a direct** connection.


## Repository Layout
```
/cmd/proxy-pool        # main
/internal/config       # CLI flags & config
/internal/fetcher      # fetch & iterate upstream list
/internal/pool         # TTL pool with round-robin
/internal/server       # HTTP proxy server (goproxy)
/internal/metrics      # optional Prometheus /metrics
Dockerfile, docker-compose.yml, build scripts
```

---

*Tip:* The original `docker-compose.yml` contains an unrelated volume mapping; use the example above instead.
