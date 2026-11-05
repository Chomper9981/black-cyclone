package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"black-cyclone/internal/nettest"
	"black-cyclone/internal/settings"
	"black-cyclone/internal/share"
	"black-cyclone/internal/singbox"
	"black-cyclone/internal/subscription"
	"black-cyclone/internal/warp"
)

/* ---------------- WARP ---------------- */

func (a *App) RegisterWarp() (settings.Settings, error) {
	a.log("Đang đăng ký tài khoản Cloudflare WARP…")
	w, err := warp.Register()
	if err != nil {
		a.log("Đăng ký WARP thất bại: " + err.Error())
		s, _ := settings.Load()
		return s, err
	}
	s, err := a.mutateSettings(func(s *settings.Settings) error {
		// Keep a previously scanned endpoint.
		if s.Warp != nil && s.Warp.Endpoint != "" {
			w.Endpoint, w.EndpointPort = s.Warp.Endpoint, s.Warp.EndpointPort
		}
		s.Warp = w
		return nil
	})
	if err == nil {
		a.log("Đăng ký WARP thành công.")
	}
	return s, err
}

// ApplyWarpLicense upgrades the account with a WARP+ key.
func (a *App) ApplyWarpLicense(key string) (settings.Settings, error) {
	key = strings.TrimSpace(key)
	return a.mutateSettings(func(s *settings.Settings) error {
		if s.Warp == nil {
			return errors.New("chưa đăng ký WARP")
		}
		if err := warp.ApplyLicense(s.Warp, key); err != nil {
			return err
		}
		a.log("Đã áp dụng key WARP+, loại tài khoản: " + s.Warp.AccountType)
		return nil
	})
}

type ScanResult struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
	Ms   int64  `json:"ms"`
}

// ScanWarpEndpoints probes WARP endpoints with real WireGuard handshakes and
// returns the reachable ones, fastest first.
func (a *App) ScanWarpEndpoints() ([]ScanResult, error) {
	s, err := settings.Load()
	if err != nil {
		return nil, err
	}
	if s.Warp == nil {
		return nil, errors.New("chưa đăng ký WARP")
	}
	a.mu.Lock()
	tunneled := a.state != StateOff
	a.mu.Unlock()
	if tunneled {
		return nil, errors.New("hãy ngắt kết nối trước khi quét (đang đi qua tunnel)")
	}
	a.log("Đang quét endpoint WARP…")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var out []ScanResult
	for _, r := range warp.Scan(ctx, s.Warp, warp.DefaultCandidates(), 50) {
		if r.OK {
			out = append(out, ScanResult{IP: r.IP, Port: r.Port, Ms: r.RTT.Milliseconds()})
		}
	}
	a.log(fmt.Sprintf("Quét xong: %d endpoint phản hồi.", len(out)))
	if len(out) > 20 {
		out = out[:20]
	}
	return out, nil
}

// SetWarpEndpoint stores the endpoint picked from a scan.
func (a *App) SetWarpEndpoint(ip string, port int) (settings.Settings, error) {
	return a.mutateSettings(func(s *settings.Settings) error {
		if s.Warp == nil {
			return errors.New("chưa đăng ký WARP")
		}
		s.Warp.Endpoint, s.Warp.EndpointPort = ip, port
		return nil
	})
}

/* ---------------- Servers & subscriptions ---------------- */

// AddServers imports every supported share link found in text.
func (a *App) AddServers(text string) (settings.Settings, error) {
	links := share.ParseLinks(text)
	if len(links) == 0 {
		s, _ := settings.Load()
		return s, errors.New("không tìm thấy link hợp lệ (vless, vmess, trojan, ss, hysteria2, tuic)")
	}
	var bad []string
	s, err := a.mutateSettings(func(s *settings.Settings) error {
		added := 0
		for _, l := range links {
			if _, err := share.ParseOutbound(l, "x"); err != nil {
				bad = append(bad, fmt.Sprintf("%s: %v", share.Name(l), err))
				continue
			}
			if hasLink(s.Servers, l) {
				continue
			}
			srv := settings.Server{ID: settings.NewID(), Name: share.Name(l), Link: l}
			s.Servers = append(s.Servers, srv)
			added++
			if len(s.Servers) == 1 {
				s.SelectedServer = srv.ID
			}
		}
		if added == 0 && len(bad) > 0 {
			return errors.New("link không hợp lệ: " + bad[0])
		}
		a.log(fmt.Sprintf("Đã thêm %d server.", added))
		return nil
	})
	for _, b := range bad {
		a.log("Bỏ qua link: " + b)
	}
	return s, err
}

func hasLink(servers []settings.Server, link string) bool {
	for _, s := range servers {
		if s.Link == link {
			return true
		}
	}
	return false
}

func (a *App) RemoveServer(id string) (settings.Settings, error) {
	return a.mutateSettings(func(s *settings.Settings) error {
		out := s.Servers[:0]
		for _, srv := range s.Servers {
			if srv.ID != id {
				out = append(out, srv)
			}
		}
		s.Servers = out
		if s.SelectedServer == id {
			s.SelectedServer = settings.AutoServer
		}
		return nil
	})
}

func (a *App) RenameServer(id, name string) (settings.Settings, error) {
	return a.mutateSettings(func(s *settings.Settings) error {
		for i := range s.Servers {
			if s.Servers[i].ID == id {
				s.Servers[i].Name = strings.TrimSpace(name)
			}
		}
		return nil
	})
}

// AddSubscription fetches url and imports its servers.
func (a *App) AddSubscription(url, name string) (settings.Settings, error) {
	url = strings.TrimSpace(url)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		s, _ := settings.Load()
		return s, errors.New("URL subscription phải bắt đầu bằng http:// hoặc https://")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	a.log("Đang tải subscription…")
	fresh, err := subscription.Fetch(ctx, url)
	if err != nil {
		a.log("Tải subscription thất bại: " + err.Error())
		s, _ := settings.Load()
		return s, err
	}
	return a.mutateSettings(func(s *settings.Settings) error {
		sub := settings.Subscription{ID: settings.NewID(), Name: strings.TrimSpace(name), URL: url, Updated: time.Now().Format(time.RFC3339)}
		if sub.Name == "" {
			sub.Name = fmt.Sprintf("Subscription %d", len(s.Subscriptions)+1)
		}
		s.Subscriptions = append(s.Subscriptions, sub)
		s.Servers = subscription.Merge(s.Servers, sub.ID, fresh)
		a.log(fmt.Sprintf("Subscription %q: %d server.", sub.Name, len(fresh)))
		return nil
	})
}

// RefreshSubscriptions re-downloads every subscription; failures keep the old servers.
func (a *App) RefreshSubscriptions() (settings.Settings, error) {
	s, err := settings.Load()
	if err != nil {
		return s, err
	}
	type res struct {
		servers []settings.Server
		err     error
	}
	results := make([]res, len(s.Subscriptions))
	var wg sync.WaitGroup
	for i, sub := range s.Subscriptions {
		wg.Add(1)
		go func(i int, url string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			results[i].servers, results[i].err = subscription.Fetch(ctx, url)
		}(i, sub.URL)
	}
	wg.Wait()
	var firstErr error
	s, err = a.mutateSettings(func(s *settings.Settings) error {
		for i := range s.Subscriptions {
			if i >= len(results) {
				break
			}
			sub := &s.Subscriptions[i]
			if r := results[i]; r.err != nil {
				a.log(fmt.Sprintf("Subscription %q lỗi: %v", sub.Name, r.err))
				if firstErr == nil {
					firstErr = fmt.Errorf("%s: %w", sub.Name, r.err)
				}
			} else {
				s.Servers = subscription.Merge(s.Servers, sub.ID, r.servers)
				sub.Updated = time.Now().Format(time.RFC3339)
				a.log(fmt.Sprintf("Subscription %q: %d server.", sub.Name, len(r.servers)))
			}
		}
		if s.SelectedServer != settings.AutoServer && !hasServer(s.Servers, s.SelectedServer) {
			s.SelectedServer = settings.AutoServer
		}
		return nil
	})
	if err != nil {
		return s, err
	}
	return s, firstErr
}

func hasServer(servers []settings.Server, id string) bool {
	for _, s := range servers {
		if s.ID == id {
			return true
		}
	}
	return false
}

func (a *App) RemoveSubscription(id string) (settings.Settings, error) {
	return a.mutateSettings(func(s *settings.Settings) error {
		subs := s.Subscriptions[:0]
		for _, sub := range s.Subscriptions {
			if sub.ID != id {
				subs = append(subs, sub)
			}
		}
		s.Subscriptions = subs
		s.Servers = subscription.Merge(s.Servers, id, nil)
		if s.SelectedServer != settings.AutoServer && !hasServer(s.Servers, s.SelectedServer) {
			s.SelectedServer = settings.AutoServer
		}
		return nil
	})
}

// PingServers returns latency in ms per server ID: -1 = failed, -2 = cannot
// measure (UDP protocol while disconnected). Through the tunnel when connected
// (real HTTP round trip), otherwise a plain TCP connect.
func (a *App) PingServers() (map[string]int64, error) {
	s, err := settings.Load()
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	c := a.clash
	live := a.state == StateOn && a.conf.Mode == settings.ModeCustom
	a.mu.Unlock()

	out := make(map[string]int64, len(s.Servers))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)
	for _, srv := range s.Servers {
		wg.Add(1)
		go func(srv settings.Server) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			ms := int64(-1)
			if live && c != nil {
				if d, err := c.Delay(ctx, singbox.ServerTag(srv.ID), "https://www.gstatic.com/generate_204", 5*time.Second); err == nil {
					ms = d.Milliseconds()
				}
			} else if ob, err := share.ParseOutbound(srv.Link, "x"); err == nil {
				host, _ := ob["server"].(string)
				port, _ := ob["server_port"].(int)
				switch ob["type"] {
				case "hysteria2", "tuic":
					ms = -2
				default:
					if d, err := nettest.TCPPing(ctx, host, port, 5*time.Second); err == nil {
						ms = d.Milliseconds()
					}
				}
			}
			mu.Lock()
			out[srv.ID] = ms
			mu.Unlock()
		}(srv)
	}
	wg.Wait()
	return out, nil
}

/* ---------------- Connection check ---------------- */

type CheckResult struct {
	IP    nettest.IPInfo `json:"ip"`
	Sites []nettest.Site `json:"sites"`
}

// CheckConnection reports the public IP and which common sites load.
func (a *App) CheckConnection() CheckResult {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var r CheckResult
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); r.IP = nettest.PublicIP(ctx) }()
	go func() { defer wg.Done(); r.Sites = nettest.CheckSites(ctx, nettest.Sites) }()
	wg.Wait()
	return r
}
