package warp

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"

	"black-cyclone/internal/settings"
)

func fixedKey(seed byte) [32]byte {
	var k [32]byte
	for i := range k {
		k[i] = seed + byte(i)
	}
	k[0] &= 248
	k[31] = (k[31] & 127) | 64
	return k
}

func pubOf(t *testing.T, priv [32]byte) [32]byte {
	t.Helper()
	p, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		t.Fatal(err)
	}
	var out [32]byte
	copy(out[:], p)
	return out
}

func TestTAI64N(t *testing.T) {
	ts := tai64n(time.Unix(1, 5))
	if binary.BigEndian.Uint64(ts[:8]) != 0x400000000000000b || binary.BigEndian.Uint32(ts[8:]) != 5 {
		t.Fatalf("bad tai64n: %x", ts)
	}
}

// Builds an initiation with fixed keys and checks it from the responder's side:
// layout, mac1, and that the Noise payloads decrypt to the initiator's static key and timestamp.
func TestBuildInitiation(t *testing.T) {
	initPriv, respPriv, ephPriv := fixedKey(1), fixedKey(77), fixedKey(150)
	initPub, respPub, ephPub := pubOf(t, initPriv), pubOf(t, respPriv), pubOf(t, ephPriv)
	now := time.Unix(1700000000, 123456789)
	reserved := [3]byte{0xAB, 0xCD, 0xEF}
	const sender = 0x11223344

	msg, err := buildInitiation(initPriv, respPub, ephPriv, sender, reserved, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg) != msgInitiationSize {
		t.Fatalf("len=%d want 148", len(msg))
	}
	if msg[0] != 1 {
		t.Fatalf("type=%d", msg[0])
	}
	if !bytes.Equal(msg[1:4], reserved[:]) {
		t.Fatalf("reserved bytes = %x", msg[1:4])
	}
	if binary.LittleEndian.Uint32(msg[4:8]) != sender {
		t.Fatal("sender index mismatch")
	}
	if !bytes.Equal(msg[8:40], ephPub[:]) {
		t.Fatal("ephemeral mismatch")
	}
	if !bytes.Equal(msg[132:148], make([]byte, 16)) {
		t.Fatal("mac2 must be zero")
	}

	// Determinism.
	msg2, _ := buildInitiation(initPriv, respPub, ephPriv, sender, reserved, now)
	if !bytes.Equal(msg, msg2) {
		t.Fatal("not deterministic")
	}

	// Responder-side mac1: BLAKE2s-128(key=BLAKE2s-256("mac1----"||respPub), msg[:116]),
	// computed over the message with the Cloudflare reserved bytes zeroed (standard header).
	std := append([]byte(nil), msg...)
	copy(std[1:4], []byte{0, 0, 0})
	k := blake2s.Sum256(append([]byte("mac1----"), respPub[:]...))
	m, _ := blake2s.New128(k[:])
	m.Write(std[:116])
	if !bytes.Equal(m.Sum(nil), msg[116:132]) {
		t.Fatal("mac1 does not verify")
	}

	// Responder-side Noise consumption.
	ck := blake2s.Sum256([]byte(noiseConstruction))
	h := mixHash(ck, []byte(wgIdentifier))
	h = mixHash(h, respPub[:])
	e := msg[8:40]
	ck = kdf1(ck[:], e)
	h = mixHash(h, e)
	ss, _ := curve25519.X25519(respPriv[:], e)
	var key [32]byte
	ck, key = kdf2(ck[:], ss)
	aead, _ := chacha20poly1305.New(key[:])
	nonce := make([]byte, 12)
	static, err := aead.Open(nil, nonce, msg[40:88], h[:])
	if err != nil || !bytes.Equal(static, initPub[:]) {
		t.Fatalf("static decrypt failed: %v", err)
	}
	h = mixHash(h, msg[40:88])
	ss, _ = curve25519.X25519(respPriv[:], static)
	_, key = kdf2(ck[:], ss)
	aead, _ = chacha20poly1305.New(key[:])
	ts, err := aead.Open(nil, nonce, msg[88:116], h[:])
	want := tai64n(now)
	if err != nil || !bytes.Equal(ts, want[:]) {
		t.Fatalf("timestamp decrypt failed: %v", err)
	}
}

func TestCheckResponse(t *testing.T) {
	b := make([]byte, msgResponseSize)
	b[0] = 2
	binary.LittleEndian.PutUint32(b[8:12], 42)
	if checkResponse(b, 42) != nil {
		t.Fatal("valid response rejected")
	}
	if checkResponse(b, 43) == nil || checkResponse(b[:91], 42) == nil {
		t.Fatal("invalid response accepted")
	}
}

func TestDefaultCandidates(t *testing.T) {
	c := DefaultCandidates()
	if len(c) < 200 || len(c) > 300 {
		t.Fatalf("got %d candidates", len(c))
	}
	seen := map[Endpoint]bool{}
	for _, e := range c {
		if seen[e] {
			t.Fatalf("duplicate %v", e)
		}
		seen[e] = true
	}
}

func TestApplyLicenseNeedsDevice(t *testing.T) {
	old := &settings.Warp{PrivateKey: "x", PeerPublicKey: "y"}
	if err := ApplyLicense(old, "xxxx-xxxx-xxxx"); err == nil {
		t.Fatal("expected error for account without device id/token")
	}
}

// Hits the real Cloudflare API, so it only runs with RCV_LIVE=1.
func TestRegisterLive(t *testing.T) {
	if os.Getenv("RCV_LIVE") != "1" {
		t.Skip("set RCV_LIVE=1 to call the Cloudflare API")
	}
	w, err := Register()
	if err != nil {
		t.Fatal(err)
	}
	if w.PeerPublicKey == "" || len(w.Addresses) != 2 || w.Addresses[0] == "/32" {
		t.Fatalf("incomplete registration: %+v", w)
	}
	if w.DeviceID == "" || w.Token == "" {
		t.Fatalf("missing device id/token: %+v", w)
	}
	t.Logf("id=%s type=%s peer=%s addrs=%v reserved=%v", w.DeviceID, w.AccountType, w.PeerPublicKey, w.Addresses, w.Reserved)

	t.Run("Scan", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cands := DefaultCandidates()
		start := time.Now()
		res := Scan(ctx, w, cands, 50)
		ok := 0
		for _, r := range res {
			if r.OK {
				ok++
			}
		}
		t.Logf("scanned %d endpoints in %v, %d OK", len(res), time.Since(start).Round(time.Millisecond), ok)
		for i := 0; i < len(res) && i < 5 && res[i].OK; i++ {
			t.Logf("  %v rtt=%v", res[i].Endpoint, res[i].RTT)
		}
		if ok == 0 {
			t.Fatal("no endpoint answered the handshake")
		}
	})

	t.Run("InvalidLicense", func(t *testing.T) {
		err := ApplyLicense(w, "00000000-00000000-00000000")
		if err == nil {
			t.Fatal("expected error for invalid license")
		}
		t.Logf("error: %v", err)
	})
}
