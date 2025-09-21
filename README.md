# proxy-pool

一个带 **代理池 + 轮询** 的 HTTP/HTTPS 代理，支持**链式上游**，定期从 API 拉取上游代理并设置 TTL 过期。

## 构建
go mod tidy
make build
