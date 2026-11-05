// Package warp registers a free Cloudflare WARP device (same API as ViRb3/wgcf).
package warp

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/curve25519"

	"black-cyclone/internal/settings"
)

const (
	apiBase       = "https://api.cloudflareclient.com/v0a1922"
	clientVersion = "a-6.3-1922"
)

var httpClient = &http.Client{Timeout: 20 * time.Second}

func setAPIHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("User-Agent", "okhttp/3.12.1")
	req.Header.Set("CF-Client-Version", clientVersion)
}

type regResponse struct {
	ID      string `json:"id"`
	Token   string `json:"token"`
	Account struct {
		AccountType string `json:"account_type"`
	} `json:"account"`
	Config struct {
		ClientID string `json:"client_id"`
		Peers    []struct {
			PublicKey string `json:"public_key"`
			Endpoint  struct {
				Host string `json:"host"`
			} `json:"endpoint"`
		} `json:"peers"`
		Interface struct {
			Addresses struct {
				V4 string `json:"v4"`
				V6 string `json:"v6"`
			} `json:"addresses"`
		} `json:"interface"`
	} `json:"config"`
}

func Register() (*settings.Warp, error) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return nil, err
	}
	// Clamp per RFC 7748.
	priv[0] &= 248
	priv[31] = (priv[31] & 127) | 64
	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return nil, err
	}

	body, _ := json.Marshal(map[string]string{
		"install_id": "",
		"fcm_token":  "",
		"key":        base64.StdEncoding.EncodeToString(pub),
		"locale":     "en_US",
		"model":      "PC",
		"tos":        time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"type":       "Android",
	})
	req, err := http.NewRequest(http.MethodPost, apiBase+"/reg", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	setAPIHeaders(req)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("warp register: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("warp register: HTTP %d: %s", resp.StatusCode, raw)
	}

	var r regResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("warp register: decode: %w", err)
	}
	if len(r.Config.Peers) == 0 {
		return nil, fmt.Errorf("warp register: no peers in response")
	}

	// client_id is 3 bytes base64; WireGuard "reserved" field needs them.
	reserved := []int{0, 0, 0}
	if id, err := base64.StdEncoding.DecodeString(r.Config.ClientID); err == nil && len(id) == 3 {
		reserved = []int{int(id[0]), int(id[1]), int(id[2])}
	}

	return &settings.Warp{
		PrivateKey:    base64.StdEncoding.EncodeToString(priv[:]),
		PeerPublicKey: r.Config.Peers[0].PublicKey,
		// The returned host name is often blocked/poisoned; the anycast IP is more reliable.
		Endpoint:     "162.159.192.1",
		EndpointPort: 2408,
		Addresses: []string{
			r.Config.Interface.Addresses.V4 + "/32",
			r.Config.Interface.Addresses.V6 + "/128",
		},
		Reserved:    reserved,
		DeviceID:    r.ID,
		Token:       r.Token,
		AccountType: r.Account.AccountType,
	}, nil
}

type accountResponse struct {
	AccountType string `json:"account_type"`
	PremiumData int64  `json:"premium_data"`
	Quota       int64  `json:"quota"`
	License     string `json:"license"`
}

// ApplyLicense binds a WARP+ license key to the registered device (same flow as wgcf:
// PUT /reg/{id}/account, then GET it back) and updates w.License / w.AccountType.
func ApplyLicense(w *settings.Warp, license string) error {
	license = strings.TrimSpace(license)
	if license == "" {
		return fmt.Errorf("key WARP+ trống")
	}
	if w == nil || w.DeviceID == "" || w.Token == "" {
		return fmt.Errorf("tài khoản WARP cũ (thiếu device id/token), hãy đăng ký lại")
	}
	body, _ := json.Marshal(map[string]string{"license": license})
	raw, status, err := authRequest(http.MethodPut, w, body)
	if err != nil {
		return fmt.Errorf("áp dụng key WARP+: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("key WARP+ không hợp lệ (HTTP %d): %s", status, strings.TrimSpace(string(raw)))
	}

	raw, status, err = authRequest(http.MethodGet, w, nil)
	if err != nil {
		return fmt.Errorf("đọc thông tin tài khoản WARP: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("đọc thông tin tài khoản WARP: HTTP %d: %s", status, strings.TrimSpace(string(raw)))
	}
	var acc accountResponse
	if err := json.Unmarshal(raw, &acc); err != nil {
		return fmt.Errorf("đọc thông tin tài khoản WARP: decode: %w", err)
	}
	w.License = license
	if acc.AccountType != "" {
		w.AccountType = acc.AccountType
	}
	return nil
}

func authRequest(method string, w *settings.Warp, body []byte) ([]byte, int, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, apiBase+"/reg/"+w.DeviceID+"/account", rd)
	if err != nil {
		return nil, 0, err
	}
	setAPIHeaders(req)
	req.Header.Set("Authorization", "Bearer "+w.Token)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	return raw, resp.StatusCode, err
}
