// Package subscription downloads proxy subscriptions (base64 or plain lists of
// share links) and merges them into the saved server list.
package subscription

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"redcyclone-vpn/internal/settings"
	"redcyclone-vpn/internal/share"
)

// UserAgent is sent so providers return share links rather than a Clash YAML or HTML page.
const UserAgent = "v2rayN/6.0"

const (
	timeout = 20 * time.Second
	maxBody = 10 << 20
)

// Fetch downloads a subscription and returns its usable servers (Name, Link and a
// fresh ID set; SubID left empty — Merge sets it). Links sing-box can't use are skipped.
func Fetch(ctx context.Context, url string) ([]settings.Server, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(url), nil)
	if err != nil {
		return nil, fmt.Errorf("subscription URL is invalid: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download subscription: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("download subscription: HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("download subscription: %w", err)
	}
	return Parse(body)
}

// Parse decodes a subscription body (base64 or plain text) into servers.
func Parse(body []byte) ([]settings.Server, error) {
	body = bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM
	text := strings.TrimSpace(string(body))
	links := share.ParseLinks(text)
	if len(links) == 0 {
		if raw, err := share.DecodeBase64(text); err == nil {
			links = share.ParseLinks(string(raw))
		}
	}
	if len(links) == 0 {
		return nil, fmt.Errorf("subscription contains no share links")
	}

	var servers []settings.Server
	seen := map[string]bool{}
	var firstErr error
	for _, l := range links {
		if seen[l] {
			continue
		}
		seen[l] = true
		if _, err := share.ParseOutbound(l, "proxy"); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		servers = append(servers, settings.Server{ID: settings.NewID(), Name: share.Name(l), Link: l})
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("subscription has no usable servers (%d links): %w", len(links), firstErr)
	}
	return servers, nil
}

// Merge replaces the servers owned by subID with fresh ones. Old IDs are reused when
// the link is unchanged so the user's selection survives a refresh. The refreshed
// servers take the place of the old ones in the list (appended if there were none).
func Merge(existing []settings.Server, subID string, fresh []settings.Server) []settings.Server {
	oldIDs := map[string][]string{}
	for _, s := range existing {
		if s.SubID == subID {
			oldIDs[s.Link] = append(oldIDs[s.Link], s.ID)
		}
	}
	updated := make([]settings.Server, 0, len(fresh))
	for _, s := range fresh {
		if ids := oldIDs[s.Link]; len(ids) > 0 {
			s.ID, oldIDs[s.Link] = ids[0], ids[1:]
		} else if s.ID == "" {
			s.ID = settings.NewID()
		}
		s.SubID = subID
		updated = append(updated, s)
	}

	out := make([]settings.Server, 0, len(existing)+len(fresh))
	inserted := false
	for _, s := range existing {
		if s.SubID != subID {
			out = append(out, s)
			continue
		}
		if !inserted {
			out = append(out, updated...)
			inserted = true
		}
	}
	if !inserted {
		out = append(out, updated...)
	}
	return out
}
