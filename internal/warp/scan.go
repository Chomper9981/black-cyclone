package warp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	mrand "math/rand/v2"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"

	"black-cyclone/internal/settings"
)

// Endpoint is a WARP UDP endpoint candidate.
type Endpoint struct {
	IP   string
	Port int
}

func (e Endpoint) String() string { return net.JoinHostPort(e.IP, strconv.Itoa(e.Port)) }

// Result is the outcome of probing one endpoint.
type Result struct {
	Endpoint
	RTT time.Duration
	OK  bool
}

// ProbeTimeout is how long a probe waits for the handshake response.
const ProbeTimeout = time.Second

var warpPrefixes = []string{
	"162.159.192.", "162.159.193.", "162.159.195.",
	"188.114.96.", "188.114.97.", "188.114.98.", "188.114.99.",
}

var warpPorts = []int{
	2408, 500, 1701, 4500, 854, 859, 864, 878, 880, 890, 891, 894, 903, 908, 928, 934, 939,
	942, 943, 945, 946, 955, 968, 987, 988, 1002, 1010, 1014, 1018, 1070, 1074, 1180, 1387,
	1843, 2371, 2506, 3138, 3476, 3581, 3854, 4177, 4198, 4233, 5279, 5956, 7103, 7152,
	7156, 7281, 7559, 8319, 8742, 8854, 8886,
}

// DefaultCandidates returns a shuffled random sample (~250 probes) of WARP anycast
// endpoints: 9 hosts per /24, each with port 2408 plus 3 random other ports.
func DefaultCandidates() []Endpoint {
	const hostsPerPrefix, extraPorts = 9, 3
	var out []Endpoint
	for _, p := range warpPrefixes {
		for _, host := range mrand.Perm(254)[:hostsPerPrefix] {
			ip := p + strconv.Itoa(host+1)
			out = append(out, Endpoint{ip, 2408})
			for _, i := range mrand.Perm(len(warpPorts) - 1)[:extraPorts] {
				out = append(out, Endpoint{ip, warpPorts[i+1]})
			}
		}
	}
	mrand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

type probeKeys struct {
	priv, peer [32]byte
	reserved   [3]byte
}

func keysFrom(w *settings.Warp) (probeKeys, error) {
	var k probeKeys
	if w == nil {
		return k, fmt.Errorf("chưa có tài khoản WARP")
	}
	priv, err := base64.StdEncoding.DecodeString(w.PrivateKey)
	if err != nil || len(priv) != 32 {
		return k, fmt.Errorf("warp private key không hợp lệ")
	}
	peer, err := base64.StdEncoding.DecodeString(w.PeerPublicKey)
	if err != nil || len(peer) != 32 {
		return k, fmt.Errorf("warp peer public key không hợp lệ")
	}
	copy(k.priv[:], priv)
	copy(k.peer[:], peer)
	for i := 0; i < 3 && i < len(w.Reserved); i++ {
		k.reserved[i] = byte(w.Reserved[i])
	}
	return k, nil
}

// Scan probes candidates with real WireGuard handshake initiations, up to parallel at a
// time, and returns all results sorted with reachable endpoints first by RTT.
// If w's keys are invalid every result is returned with OK=false.
func Scan(ctx context.Context, w *settings.Warp, candidates []Endpoint, parallel int) []Result {
	results := make([]Result, len(candidates))
	for i, c := range candidates {
		results[i].Endpoint = c
	}
	keys, err := keysFrom(w)
	if err != nil {
		return results
	}
	if parallel <= 0 {
		parallel = 50
	}

	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup
loop:
	for i := range candidates {
		select {
		case <-ctx.Done():
			break loop
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			rtt, err := probe(ctx, keys, candidates[i])
			if err == nil {
				results[i].RTT, results[i].OK = rtt, true
			}
		}(i)
	}
	wg.Wait()

	sort.SliceStable(results, func(a, b int) bool {
		ra, rb := results[a], results[b]
		if ra.OK != rb.OK {
			return ra.OK
		}
		return ra.OK && ra.RTT < rb.RTT
	})
	return results
}

func probe(ctx context.Context, k probeKeys, ep Endpoint) (time.Duration, error) {
	eph, err := newEphemeral()
	if err != nil {
		return 0, err
	}
	var sb [4]byte
	if _, err := rand.Read(sb[:]); err != nil {
		return 0, err
	}
	sender := binary.LittleEndian.Uint32(sb[:])
	msg, err := buildInitiation(k.priv, k.peer, eph, sender, k.reserved, time.Now())
	if err != nil {
		return 0, err
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", ep.String())
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	deadline := time.Now().Add(ProbeTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { conn.SetDeadline(time.Now()) })
	defer stop()

	start := time.Now()
	if _, err := conn.Write(msg); err != nil {
		return 0, err
	}
	buf := make([]byte, 1500)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return 0, err
		}
		if checkResponse(buf[:n], sender) == nil {
			return time.Since(start), nil
		}
	}
}
