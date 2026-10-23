// Package clash talks to sing-box's Clash-compatible API (experimental.clash_api):
// traffic counters, switching the selected server and latency tests.
package clash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	Addr   string // "127.0.0.1:port"
	Secret string
	http   *http.Client
}

// FreeAddr picks an unused loopback TCP address for the controller.
func FreeAddr() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	return l.Addr().String(), nil
}

func New(addr, secret string) *Client {
	// Loopback only; never route through a proxy.
	tr := &http.Transport{Proxy: nil}
	return &Client{Addr: addr, Secret: secret, http: &http.Client{Transport: tr, Timeout: 15 * time.Second}}
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://"+c.Addr+path, rd)
	if err != nil {
		return err
	}
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("clash api %s %s: HTTP %d: %s", method, path, resp.StatusCode, bytes.TrimSpace(msg))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// Totals returns cumulative upload/download bytes since sing-box started.
func (c *Client) Totals(ctx context.Context) (up, down int64, err error) {
	var r struct {
		UploadTotal   int64 `json:"uploadTotal"`
		DownloadTotal int64 `json:"downloadTotal"`
	}
	err = c.do(ctx, http.MethodGet, "/connections", nil, &r)
	return r.UploadTotal, r.DownloadTotal, err
}

// Select switches a selector outbound (e.g. "proxy") to the given member tag.
func (c *Client) Select(ctx context.Context, selector, member string) error {
	return c.do(ctx, http.MethodPut, "/proxies/"+url.PathEscape(selector), map[string]string{"name": member}, nil)
}

// Now returns the member a selector/urltest outbound currently uses.
func (c *Client) Now(ctx context.Context, group string) (string, error) {
	var r struct {
		Now string `json:"now"`
	}
	err := c.do(ctx, http.MethodGet, "/proxies/"+url.PathEscape(group), nil, &r)
	return r.Now, err
}

// Delay measures the round trip of an HTTP request through the given outbound.
func (c *Client) Delay(ctx context.Context, tag, testURL string, timeout time.Duration) (time.Duration, error) {
	var r struct {
		Delay int `json:"delay"`
	}
	q := url.Values{"url": {testURL}, "timeout": {fmt.Sprint(timeout.Milliseconds())}}
	if err := c.do(ctx, http.MethodGet, "/proxies/"+url.PathEscape(tag)+"/delay?"+q.Encode(), nil, &r); err != nil {
		return 0, err
	}
	return time.Duration(r.Delay) * time.Millisecond, nil
}
