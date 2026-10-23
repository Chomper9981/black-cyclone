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
	"time"

	"golang.org/x/crypto/curve25519"

	"redcyclone-vpn/internal/settings"
)

const (
	apiBase       = "https://api.cloudflareclient.com/v0a1922"
	clientVersion = "a-6.3-1922"
)

type regResponse struct {
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
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("User-Agent", "okhttp/3.12.1")
	req.Header.Set("CF-Client-Version", clientVersion)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
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
		Reserved: reserved,
	}, nil
}
