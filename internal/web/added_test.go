package web

import (
	"testing"
	"time"
)

func TestAddedLabel(t *testing.T) {
	cest := time.FixedZone("CEST", 2*3600)
	now := time.Date(2026, 10, 1, 14, 40, 0, 0, cest)
	cases := []struct {
		created time.Time
		want    string
	}{
		{time.Date(2026, 10, 1, 14, 32, 0, 0, cest), "Added today, 14:32"},
		{time.Date(2026, 9, 30, 9, 10, 0, 0, cest), "Added yesterday, 09:10"},
		{time.Date(2026, 9, 30, 23, 59, 0, 0, cest), "Added yesterday, 23:59"},
		{time.Date(2026, 9, 30, 23, 30, 0, 0, time.UTC), "Added today, 01:30"},
		{time.Date(2026, 9, 28, 8, 0, 0, 0, cest), "Added 28 Sep"},
		{time.Date(2025, 12, 3, 8, 0, 0, 0, cest), "Added 3 Dec 2025"},
		{time.Date(2026, 10, 2, 8, 0, 0, 0, cest), "Added 2 Oct"},
	}
	for _, c := range cases {
		if got := addedLabel(c.created, now); got != c.want {
			t.Errorf("addedLabel(%v) = %q, want %q", c.created, got, c.want)
		}
	}
}
