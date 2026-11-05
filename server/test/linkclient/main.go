// Command linkclient writes a minimal sing-box client config for a share link,
// using the app's own link parser: a SOCKS/HTTP proxy on 127.0.0.1:<port> that
// sends everything through the link's server. server/test/run.ps1 uses it to
// check that the links bcserver prints actually work with Black Cyclone.
//
// Usage: go run ./server/test/linkclient <link> <port> <out.json>
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"black-cyclone/internal/share"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: linkclient <link> <port> <out.json>")
		os.Exit(2)
	}
	port, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fail(err)
	}
	ob, err := share.ParseOutbound(os.Args[1], "proxy")
	if err != nil {
		fail(err)
	}
	cfg := map[string]any{
		"log":       map[string]any{"level": "warn"},
		"inbounds":  []any{map[string]any{"type": "mixed", "listen": "127.0.0.1", "listen_port": port}},
		"outbounds": []any{ob},
		"route":     map[string]any{"final": "proxy"},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(os.Args[3], data, 0o600); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "linkclient:", err)
	os.Exit(1)
}
