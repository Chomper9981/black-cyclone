package clash

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Runs a real sing-box (no TUN) with the Clash API and exercises the client.
func TestAgainstSingBox(t *testing.T) {
	bin, _ := filepath.Abs(filepath.Join("..", "..", "bin", "sing-box.exe"))
	if _, err := os.Stat(bin); err != nil {
		t.Skip("bin/sing-box.exe not found")
	}
	addr, err := FreeAddr()
	if err != nil {
		t.Fatal(err)
	}
	cfg := fmt.Sprintf(`{
  "log": {"level": "warn"},
  "outbounds": [
    {"type": "direct", "tag": "a"},
    {"type": "direct", "tag": "b"},
    {"type": "selector", "tag": "proxy", "outbounds": ["a", "b"], "default": "a"}
  ],
  "experimental": {"clash_api": {"external_controller": %q, "secret": "s3cret"}}
}`, addr)
	f := filepath.Join(t.TempDir(), "c.json")
	if err := os.WriteFile(f, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "run", "-c", f)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()

	c := New(addr, "s3cret")
	ctx := context.Background()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, _, err = c.Totals(ctx); err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Select(ctx, "proxy", "b"); err != nil {
		t.Fatal(err)
	}
	if now, err := c.Now(ctx, "proxy"); err != nil || now != "b" {
		t.Fatalf("now=%q err=%v", now, err)
	}
	if err := New(addr, "wrong").Select(ctx, "proxy", "a"); err == nil {
		t.Fatal("wrong secret accepted")
	}
}
