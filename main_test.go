package main

import (
	"path/filepath"
	"testing"

	qrcodegen "github.com/skip2/go-qrcode"
)

func TestPayloadRoundTrip(t *testing.T) {
	in := network{SSID: `a;b:c,"d\`, Password: `p;w:`, Auth: "WPA", Hidden: true}
	out, err := parsePayload(in.payload())
	if err != nil || out != in {
		t.Fatalf("got %+v, %v; want %+v", out, err, in)
	}
}

func TestImageRoundTrip(t *testing.T) {
	want := network{SSID: "home", Password: "secret", Auth: "WPA"}.payload()
	path := filepath.Join(t.TempDir(), "w.png")
	if err := qrcodegen.WriteFile(want, qrcodegen.Medium, 512, path); err != nil {
		t.Fatal(err)
	}
	got, err := decodeImage(path)
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}
