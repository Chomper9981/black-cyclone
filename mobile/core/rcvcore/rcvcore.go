// Package rcvcore is the gomobile-bound API used by the Android app (Java class
// rcvcore.Rcvcore). Only gomobile-friendly types cross the boundary: structured
// values travel as JSON strings.
package rcvcore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"blackcyclone-vpn-mobile/core/tunnel"
	"blackcyclone-vpn-mobile/core/warp"
)

var (
	mu  sync.Mutex
	cur *tunnel.Tunnel

	scanMu     sync.Mutex
	scanCancel context.CancelFunc
)

func parseAccount(accountJSON string) (*warp.Account, error) {
	var a warp.Account
	if err := json.Unmarshal([]byte(accountJSON), &a); err != nil {
		return nil, fmt.Errorf("tài khoản WARP hỏng: %w", err)
	}
	return &a, nil
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Register creates a new free WARP device and returns it as account JSON.
func Register() (string, error) {
	a, err := warp.Register()
	if err != nil {
		return "", err
	}
	logf("Đã đăng ký WARP mới (%s)", a.AccountType)
	return toJSON(a), nil
}

// ApplyLicense binds a WARP+ key to the account and returns the updated account JSON.
func ApplyLicense(accountJSON, license string) (string, error) {
	a, err := parseAccount(accountJSON)
	if err != nil {
		return "", err
	}
	if err := warp.ApplyLicense(a, license); err != nil {
		return "", err
	}
	logf("Đã áp dụng key WARP+ (%s)", a.AccountType)
	return toJSON(a), nil
}

type scanResult struct {
	IP    string `json:"ip"`
	Port  int    `json:"port"`
	RTTMs int64  `json:"rtt_ms"`
}

type scanReport struct {
	Scanned int          `json:"scanned"`
	OK      []scanResult `json:"ok"` // fastest first, at most 20
}

// Scan probes random WARP endpoints with real WireGuard handshakes for up to
// timeoutSec seconds and returns a scanReport JSON. CancelScan stops it early.
func Scan(accountJSON string, parallel, timeoutSec int) (string, error) {
	a, err := parseAccount(accountJSON)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()
	scanMu.Lock()
	if scanCancel != nil {
		scanCancel()
	}
	scanCancel = cancel
	scanMu.Unlock()

	cands := warp.DefaultCandidates()
	res := warp.Scan(ctx, a, cands, parallel)
	rep := scanReport{Scanned: len(cands), OK: []scanResult{}}
	for _, r := range res {
		if !r.OK || len(rep.OK) >= 20 {
			break
		}
		rep.OK = append(rep.OK, scanResult{r.IP, r.Port, r.RTT.Milliseconds()})
	}
	logf("Quét %d endpoint: %d phản hồi", len(cands), countOK(res))
	return toJSON(rep), nil
}

func countOK(res []warp.Result) int {
	n := 0
	for _, r := range res {
		if r.OK {
			n++
		}
	}
	return n
}

// CancelScan stops a running Scan; it returns what it has found so far.
func CancelScan() {
	scanMu.Lock()
	defer scanMu.Unlock()
	if scanCancel != nil {
		scanCancel()
		scanCancel = nil
	}
}

// Start runs WARP on the TUN fd from VpnService.Builder.establish().detachFd().
// The fd is owned by the tunnel from here on, also on error.
func Start(fd int, accountJSON string) error {
	a, err := parseAccount(accountJSON)
	if err != nil {
		os.NewFile(uintptr(fd), "tun").Close()
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	if cur != nil {
		cur.Close()
		cur = nil
	}
	t, err := tunnel.Start(fd, a, func(s string) {
		if !strings.Contains(s, "keepalive") {
			logf("%s", s)
		}
	})
	if err != nil {
		logf("Không khởi động được: %v", err)
		return err
	}
	cur = t
	logf("Đã bật WARP qua %s:%d", a.Endpoint, a.EndpointPort)
	return nil
}

// Stop shuts the tunnel down.
func Stop() {
	mu.Lock()
	defer mu.Unlock()
	if cur != nil {
		cur.Close()
		cur = nil
		logf("Đã tắt WARP")
	}
}

// Rebind reopens the tunnel's UDP sockets after a network change.
func Rebind() {
	mu.Lock()
	defer mu.Unlock()
	if cur != nil {
		if err := cur.Rebind(); err != nil {
			logf("Rebind: %v", err)
		}
	}
}

type statsJSON struct {
	Running bool  `json:"running"`
	Rx      int64 `json:"rx"`
	Tx      int64 `json:"tx"`
	// Milliseconds since the last handshake, -1 if none yet.
	HandshakeAgoMs int64 `json:"handshake_ago_ms"`
}

// Stats returns traffic counters as JSON.
func Stats() string {
	mu.Lock()
	t := cur
	mu.Unlock()
	out := statsJSON{HandshakeAgoMs: -1}
	if t == nil {
		return toJSON(out)
	}
	s, err := t.Stats()
	if err != nil {
		return toJSON(out)
	}
	out.Running, out.Rx, out.Tx = true, s.RxBytes, s.TxBytes
	if !s.LastHandshake.IsZero() {
		out.HandshakeAgoMs = time.Since(s.LastHandshake).Milliseconds()
	}
	return toJSON(out)
}
