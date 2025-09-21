package fetcher

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type Fetcher struct {
	apiURL string
	client *http.Client

	cache  []string
	cursor int
}

func New(apiURL string, dialTimeout time.Duration) *Fetcher {
	return &Fetcher{
		apiURL: apiURL,
		client: &http.Client{Timeout: dialTimeout},
	}
}

// FetchList 拉取一次 API，支持 JSON 数组或换行文本
func (f *Fetcher) FetchList(ctx context.Context) ([]string, error) {
	if f.apiURL == "" {
		return nil, errors.New("empty api url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.apiURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, errors.New("bad status: " + resp.Status + " body: " + string(b))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 优先解析 JSON
	var arr []string
	if err := json.Unmarshal(body, &arr); err == nil && len(arr) > 0 {
		return normalize(arr), nil
	}

	// 退化为按行文本
	return readLines(string(body)), nil
}

func readLines(s string) []string {
	var res []string
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			res = append(res, line)
		}
	}
	return normalize(res)
}

func normalize(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		// 不再强行补 "http://"
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// Next 每次返回一个地址；缓存耗尽则自动重新拉取
func (f *Fetcher) Next(ctx context.Context) (string, error) {
	if f.cursor < len(f.cache) {
		addr := f.cache[f.cursor]
		f.cursor++
		return addr, nil
	}
	list, err := f.FetchList(ctx)
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", errors.New("empty proxy list from api")
	}
	f.cache = list
	f.cursor = 0
	addr := f.cache[f.cursor]
	f.cursor++
	return addr, nil
}
