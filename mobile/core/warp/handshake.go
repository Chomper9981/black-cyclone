package warp

import (
	"crypto/hmac"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"hash"
	"time"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

// Minimal WireGuard handshake initiation (Noise_IKpsk2_25519_ChaChaPoly_BLAKE2s),
// just enough to make a WARP endpoint answer with a handshake response.

const (
	noiseConstruction = "Noise_IKpsk2_25519_ChaChaPoly_BLAKE2s"
	wgIdentifier      = "WireGuard v1 zx2c4 Jason@zx2c4.com"
	wgLabelMAC1       = "mac1----"

	msgInitiationType = 1
	msgResponseType   = 2
	msgInitiationSize = 148
	msgResponseSize   = 92
	mac1Offset        = 116 // type(1)+reserved(3)+sender(4)+ephemeral(32)+static(48)+timestamp(28)
)

func newHMAC(key []byte) hash.Hash {
	return hmac.New(func() hash.Hash {
		h, _ := blake2s.New256(nil)
		return h
	}, key)
}

func hmacSum(key []byte, in ...[]byte) [32]byte {
	m := newHMAC(key)
	for _, b := range in {
		m.Write(b)
	}
	var out [32]byte
	m.Sum(out[:0])
	return out
}

func kdf1(key, input []byte) [32]byte {
	t0 := hmacSum(key, input)
	return hmacSum(t0[:], []byte{1})
}

func kdf2(key, input []byte) (t1, t2 [32]byte) {
	t0 := hmacSum(key, input)
	t1 = hmacSum(t0[:], []byte{1})
	t2 = hmacSum(t0[:], t1[:], []byte{2})
	return
}

func mixHash(h [32]byte, data []byte) [32]byte {
	d, _ := blake2s.New256(nil)
	d.Write(h[:])
	d.Write(data)
	var out [32]byte
	d.Sum(out[:0])
	return out
}

// mac1Key = BLAKE2s-256("mac1----" || responderPub).
func mac1Key(responderPub [32]byte) [32]byte {
	d, _ := blake2s.New256(nil)
	d.Write([]byte(wgLabelMAC1))
	d.Write(responderPub[:])
	var out [32]byte
	d.Sum(out[:0])
	return out
}

func computeMAC1(responderPub [32]byte, msg []byte) [16]byte {
	k := mac1Key(responderPub)
	m, _ := blake2s.New128(k[:])
	m.Write(msg)
	var out [16]byte
	m.Sum(out[:0])
	return out
}

// tai64n returns the 12-byte TAI64N label for t.
func tai64n(t time.Time) [12]byte {
	var out [12]byte
	binary.BigEndian.PutUint64(out[:8], uint64(0x400000000000000a)+uint64(t.Unix()))
	binary.BigEndian.PutUint32(out[8:], uint32(t.Nanosecond()))
	return out
}

func aeadSeal(key [32]byte, plaintext, ad []byte) []byte {
	a, _ := chacha20poly1305.New(key[:])
	var nonce [chacha20poly1305.NonceSize]byte // counter 0
	return a.Seal(nil, nonce[:], plaintext, ad)
}

// buildInitiation constructs a 148-byte handshake initiation from staticPriv to peerPub.
// ephPriv must be a clamped X25519 private key. reserved (Cloudflare client_id) is written
// into bytes 1..3 after the MACs are computed, as Cloudflare's WARP clients do; mac1 thus
// covers the message with zeroed reserved bytes (as in standard WireGuard).
func buildInitiation(staticPriv, peerPub, ephPriv [32]byte, sender uint32, reserved [3]byte, now time.Time) ([]byte, error) {
	staticPub, err := curve25519.X25519(staticPriv[:], curve25519.Basepoint)
	if err != nil {
		return nil, err
	}
	ephPub, err := curve25519.X25519(ephPriv[:], curve25519.Basepoint)
	if err != nil {
		return nil, err
	}

	ck := blake2s.Sum256([]byte(noiseConstruction))
	h := mixHash(ck, []byte(wgIdentifier))
	h = mixHash(h, peerPub[:])

	msg := make([]byte, msgInitiationSize)
	msg[0] = msgInitiationType
	binary.LittleEndian.PutUint32(msg[4:8], sender)
	copy(msg[8:40], ephPub)

	ck = kdf1(ck[:], ephPub)
	h = mixHash(h, ephPub)

	ss, err := curve25519.X25519(ephPriv[:], peerPub[:])
	if err != nil {
		return nil, err
	}
	var key [32]byte
	ck, key = kdf2(ck[:], ss)
	encStatic := aeadSeal(key, staticPub, h[:])
	copy(msg[40:88], encStatic)
	h = mixHash(h, encStatic)

	ss, err = curve25519.X25519(staticPriv[:], peerPub[:])
	if err != nil {
		return nil, err
	}
	ck, key = kdf2(ck[:], ss)
	ts := tai64n(now)
	encTS := aeadSeal(key, ts[:], h[:])
	copy(msg[88:116], encTS)

	mac1 := computeMAC1(peerPub, msg[:mac1Offset])
	copy(msg[mac1Offset:mac1Offset+16], mac1[:])
	// mac2 stays zero (no cookie).

	copy(msg[1:4], reserved[:])
	return msg, nil
}

func newEphemeral() ([32]byte, error) {
	var k [32]byte
	if _, err := rand.Read(k[:]); err != nil {
		return k, err
	}
	k[0] &= 248
	k[31] = (k[31] & 127) | 64
	return k, nil
}

var errNotResponse = errors.New("not a handshake response")

// checkResponse validates that b looks like a handshake response addressed to sender.
func checkResponse(b []byte, sender uint32) error {
	if len(b) != msgResponseSize || b[0] != msgResponseType {
		return errNotResponse
	}
	if binary.LittleEndian.Uint32(b[8:12]) != sender {
		return errNotResponse
	}
	return nil
}
