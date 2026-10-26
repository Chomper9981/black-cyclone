package runner

import (
	"slices"
	"testing"
)

func processAlive(pid int) bool { return alive(pid) }

func TestWithoutPath(t *testing.T) {
	got := withoutPath([]string{"HOME=/h", "PATH=/usr/bin", "PATHEXT=x"})
	if !slices.Equal(got, []string{"HOME=/h", "PATHEXT=x"}) {
		t.Fatalf("got %q", got)
	}
}
