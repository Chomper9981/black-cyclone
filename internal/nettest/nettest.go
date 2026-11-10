// Package nettest checks what the outside world sees: public IP and whether
// commonly blocked sites are reachable. Requests go through the system route,
// i.e. through the TUN when connected.
package nettest

import (
	"bufio"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"black-cyclone/internal/i18n"
)

type IPInfo struct {
	IP      string `json:"ip"`
	Country string `json:"country"`
	Colo    string `json:"colo"` // Cloudflare data centre that answered
	WARP    string `json:"warp"` // "off", "on", "plus"
	Error   string `json:"error"`
}

type Site struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	OK     bool   `json:"ok"`
	Status int    `json:"status"`
	Ms     int64  `json:"ms"`
	Error  string `json:"error"`
}

// Sites are commonly blocked or throttled targets in Vietnam plus a baseline.
var Sites = []Site{
	{Name: "Google", URL: "https://www.google.com/generate_204"},
	{Name: "YouTube", URL: "https://www.youtube.com/"},
	{Name: "Facebook", URL: "https://www.facebook.com/"},
	{Name: "Telegram", URL: "https://web.telegram.org/"},
	{Name: "Discord", URL: "https://discord.com/"},
	{Name: "Medium", URL: "https://medium.com/"},
	{Name: "Steam", URL: "https://store.steampowered.com/"},
	{Name: "VnExpress", URL: "https://vnexpress.net/"},
}

func client(timeout time.Duration) *http.Client {
	tr := &http.Transport{
		Proxy:               nil,
		TLSHandshakeTimeout: timeout,
		DisableKeepAlives:   true,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
	}
	return &http.Client{
		Transport: tr,
		Timeout:   timeout,
		// A redirect still proves the site is reachable.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// PublicIP queries Cloudflare's trace endpoint.
func PublicIP(ctx context.Context) IPInfo {
	var info IPInfo
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://1.1.1.1/cdn-cgi/trace", nil)
	resp, err := client(8 * time.Second).Do(req)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(io.LimitReader(resp.Body, 8<<10))
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if !ok {
			continue
		}
		switch k {
		case "ip":
			info.IP = v
		case "loc":
			info.Country = v
		case "colo":
			info.Colo = v
		case "warp":
			info.WARP = v
		}
	}
	return info
}

// CheckSites fetches every site in parallel.
func CheckSites(ctx context.Context, sites []Site) []Site {
	out := make([]Site, len(sites))
	c := client(10 * time.Second)
	var wg sync.WaitGroup
	for i, s := range sites {
		wg.Add(1)
		go func(i int, s Site) {
			defer wg.Done()
			start := time.Now()
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
			resp, err := c.Do(req)
			s.Ms = time.Since(start).Milliseconds()
			if err != nil {
				s.Error = shortErr(err)
			} else {
				resp.Body.Close()
				s.Status = resp.StatusCode
				s.OK = resp.StatusCode < 500
			}
			out[i] = s
		}(i, s)
	}
	wg.Wait()
	return out
}

// TCPPing measures how long a TCP connect to host:port takes.
func TCPPing(ctx context.Context, host string, port int, timeout time.Duration) (time.Duration, error) {
	d := net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return 0, err
	}
	conn.Close()
	return time.Since(start), nil
}

func shortErr(err error) string {
	s := err.Error()
	switch {
	case strings.Contains(s, "timeout"), strings.Contains(s, "deadline"):
		return i18n.T("hết thời gian chờ", "timed out")
	case strings.Contains(s, "connection reset"), strings.Contains(s, "forcibly closed"):
		return i18n.T("bị ngắt kết nối (có thể bị chặn)", "connection reset (possibly blocked)")
	case strings.Contains(s, "no such host"):
		return i18n.T("không phân giải được tên miền", "could not resolve the domain")
	case strings.Contains(s, "certificate"):
		return i18n.T("chứng chỉ TLS sai (có thể bị chặn)", "wrong TLS certificate (possibly blocked)")
	}
	if len(s) > 80 {
		s = s[:80] + "…"
	}
	return s
}
