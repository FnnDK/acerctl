package hw

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestParseTestKey(t *testing.T) {
	cases := map[string]string{
		"/etc/pki/akmods/certs/public_key.der is already enrolled\n":                  "enrolled",
		"/etc/pki/akmods/certs/public_key.der is not enrolled\n":                      "notenrolled",
		"/etc/pki/akmods/certs/public_key.der is already in the enrollment request\n": "pending",
		"Failed to open /etc/pki/akmods/certs/public_key.der\n":                       "",
	}
	for out, want := range cases {
		if got := parseTestKey(out); got != want {
			t.Errorf("%q: got %q, want %q", out, got, want)
		}
	}
}

func TestPredatorV4In(t *testing.T) {
	dir := t.TempDir()
	write := func(name, s string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("other.conf", "options snd_hda_intel power_save=1\n# options acer_wmi predator_v4=1\n")
	write("acer.txt", "options acer_wmi predator_v4=1\n") // not a .conf: modprobe ignores it
	if predatorV4In(dir) {
		t.Fatal("found predator_v4 in a comment or a non-.conf file")
	}
	write("acer-wmi.conf", "options acer_wmi predator_v4=1 force_caps=0x1C26\n")
	if !predatorV4In(dir) {
		t.Fatal("missed options acer_wmi predator_v4=1")
	}
}

func TestOneTimeCode(t *testing.T) {
	for i := 0; i < 50; i++ {
		c, err := oneTimeCode()
		if err != nil || !regexp.MustCompile(`^[1-9][0-9]$`).MatchString(c) {
			t.Fatalf("code %q, %v", c, err)
		}
	}
}

func TestSimSetup(t *testing.T) {
	s := SimSetup()
	if len(s.Items()) != 2 {
		t.Fatalf("items %v", s.Items())
	}
	code, err := s.Apply("signing_key")
	if err != nil || len(code) != 2 {
		t.Fatalf("code %q, %v", code, err)
	}
	for _, it := range s.Items() {
		if it.ID == "signing_key" && it.State != "reboot" {
			t.Fatalf("state %q after apply", it.State)
		}
	}
}
