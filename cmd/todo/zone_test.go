package main

import "testing"

func TestServerZone(t *testing.T) {
	t.Setenv("TZ", "Asia/Tokyo")
	if z := serverZone(); z.String() != "Asia/Tokyo" {
		t.Fatalf("TZ=Asia/Tokyo: %s", z)
	}
	t.Setenv("TZ", ":Europe/Warsaw")
	if z := serverZone(); z.String() != "Europe/Warsaw" {
		t.Fatalf("TZ=:Europe/Warsaw: %s", z)
	}
	t.Setenv("TZ", "Mars/Base")
	if z := serverZone(); z.String() == "Mars/Base" {
		t.Fatal("an unknown TZ must not be used")
	}
}
