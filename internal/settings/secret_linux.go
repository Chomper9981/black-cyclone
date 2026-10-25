package settings

import (
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/crypto/chacha20poly1305"
)

// Linux has no DPAPI. settings.json is sealed with XChaCha20-Poly1305 under a
// random per-user key stored next to it (mode 0600). That keeps the secrets
// out of a settings.json copied on its own (bug reports, backups, sync); it
// does not protect against someone who can read the whole folder.

const keyFile = "settings.key"

func protect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	c, err := aead(true)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, c.NonceSize(), c.NonceSize()+len(data)+c.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return c.Seal(nonce, nonce, data, nil), nil
}

func unprotect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	c, err := aead(false)
	if err != nil {
		return nil, err
	}
	n := c.NonceSize()
	if len(data) < n {
		return nil, errors.New("dữ liệu mã hoá bị cụt")
	}
	return c.Open(nil, data[:n], data[n:], nil)
}

// aead loads the key, creating it first if create is set and none exists.
func aead(create bool) (cipher.AEAD, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	p := filepath.Join(dir, keyFile)
	key, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) && create {
		key = make([]byte, chacha20poly1305.KeySize)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		// O_EXCL: if another writer won the race, use its key instead.
		f, ferr := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if ferr == nil {
			_, werr := f.Write(key)
			if err := errors.Join(werr, f.Close()); err != nil {
				_ = os.Remove(p)
				return nil, err
			}
		} else if key, err = os.ReadFile(p); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, fmt.Errorf("khoá %s: %w", keyFile, err)
	}
	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("khoá %s không hợp lệ", keyFile)
	}
	return chacha20poly1305.NewX(key)
}
