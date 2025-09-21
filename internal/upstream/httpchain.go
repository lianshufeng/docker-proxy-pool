// placeholder httpchain.go
package upstream

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"
)

func parseUpstream(addr string) (*url.URL, error) {
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return url.Parse(addr)
	}
	return url.Parse("http://" + addr)
}

// 通过“上游 HTTP 代理”建立到 target 的 CONNECT 隧道
func DialViaHTTPProxy(ctx context.Context, upstreamAddr, target string, timeout time.Duration) (net.Conn, error) {
	u, err := parseUpstream(upstreamAddr)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("bad upstream: %v", err)
	}
	d := &net.Dialer{Timeout: timeout}
	upConn, err := d.DialContext(ctx, "tcp", u.Host)
	if err != nil {
		return nil, err
	}

	// 写 CONNECT
	var b strings.Builder
	fmt.Fprintf(&b, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n", target, target)
	if u.User != nil {
		if pw, ok := u.User.Password(); ok {
			creds := u.User.Username() + ":" + pw
			auth := "Basic " + base64Std(creds)
			fmt.Fprintf(&b, "Proxy-Authorization: %s\r\n", auth)
		}
	}
	b.WriteString("\r\n")
	if _, err = io.WriteString(upConn, b.String()); err != nil {
		upConn.Close()
		return nil, err
	}
	// 读应答
	br := bufio.NewReader(upConn)
	line, err := br.ReadString('\n')
	if err != nil || !strings.Contains(line, "200") {
		upConn.Close()
		if err == nil {
			err = fmt.Errorf("CONNECT failed: %q", strings.TrimSpace(line))
		}
		return nil, err
	}
	// 扔掉头
	for {
		h, _ := br.ReadString('\n')
		if h == "\r\n" || h == "\n" || h == "" {
			break
		}
	}
	return upConn, nil
}

func base64Std(s string) string {
	const enc = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var out strings.Builder
	var val uint
	var valb int
	for i := 0; i < len(s); i++ {
		val = (val << 8) | uint(s[i])
		valb += 8
		for valb >= 6 {
			out.WriteByte(enc[(val>>(uint(valb-6)))&0x3F])
			valb -= 6
		}
	}
	if valb > 0 {
		out.WriteByte(enc[(val<<(uint(6-valb)))&0x3F])
	}
	for out.Len()%4 != 0 {
		out.WriteByte('=')
	}
	return out.String()
}
