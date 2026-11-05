package subscription

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"black-cyclone/internal/settings"
)

const (
	linkA = "vless://11111111-2222-3333-4444-555555555555@a.example.com:443?security=tls&type=ws&path=%2Fws#Node%20A"
	linkB = "trojan://pass@b.example.com:443#Node B"
	linkX = "vless://11111111-2222-3333-4444-555555555555@x.example.com:443?type=xhttp#unsupported"
)

func serve(t *testing.T, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, "v2rayN") {
			t.Errorf("User-Agent = %q", ua)
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestFetch(t *testing.T) {
	plain := "# my sub\n" + linkA + "\r\n\r\n" + linkB + "\n" + linkX + "\n" + linkA + "\n"
	b64 := base64.StdEncoding.EncodeToString([]byte(plain))
	wrapped := ""
	for i := 0; i < len(b64); i += 76 {
		wrapped += b64[i:min(i+76, len(b64))] + "\n"
	}
	cases := map[string]string{
		"plain":       plain,
		"base64":      b64,
		"base64-wrap": wrapped,
		"base64-url":  base64.RawURLEncoding.EncodeToString([]byte(plain)),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := Fetch(context.Background(), serve(t, body))
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 2 {
				t.Fatalf("got %d servers: %+v", len(got), got)
			}
			if got[0].Name != "Node A" || got[0].Link != linkA || got[1].Name != "Node B" {
				t.Errorf("unexpected servers: %+v", got)
			}
			if got[0].ID == "" || got[0].ID == got[1].ID {
				t.Errorf("bad IDs: %+v", got)
			}
		})
	}
}

func TestFetchErrors(t *testing.T) {
	for name, body := range map[string]string{
		"empty":       "",
		"html":        "<html>nope</html>",
		"unsupported": linkX,
	} {
		if _, err := Fetch(context.Background(), serve(t, body)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer srv.Close()
	if _, err := Fetch(context.Background(), srv.URL); err == nil {
		t.Error("expected HTTP error")
	}
}

func TestMerge(t *testing.T) {
	existing := []settings.Server{
		{ID: "m1", Name: "manual", Link: "trojan://x@m:1"},
		{ID: "s1", Name: "A", Link: linkA, SubID: "sub"},
		{ID: "s2", Name: "gone", Link: "trojan://old@o:1", SubID: "sub"},
		{ID: "o1", Name: "other", Link: linkA, SubID: "other"},
		{ID: "m2", Name: "manual2", Link: "trojan://y@m:2"},
	}
	fresh := []settings.Server{
		{ID: "n1", Name: "A renamed", Link: linkA},
		{ID: "n2", Name: "B", Link: linkB},
	}
	got := Merge(existing, "sub", fresh)
	ids := []string{}
	for _, s := range got {
		ids = append(ids, s.ID+"/"+s.SubID)
	}
	want := "m1/ s1/sub n2/sub o1/other m2/"
	if strings.Join(ids, " ") != want {
		t.Fatalf("got %q, want %q", strings.Join(ids, " "), want)
	}
	if got[1].Name != "A renamed" {
		t.Errorf("name not refreshed: %+v", got[1])
	}

	// New subscription: appended at the end.
	got = Merge(existing[:1], "new", fresh)
	if len(got) != 3 || got[2].SubID != "new" || got[1].ID != "n1" {
		t.Errorf("append case: %+v", got)
	}
}
