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

func TestIsDKMSPath(t *testing.T) {
	for p, want := range map[string]bool{
		"/lib/modules/6.12.48+deb13-amd64/updates/dkms/acer-wmi-extras.ko.xz\n":  true,
		"/lib/modules/7.0.0-38-generic/updates/dkms/acer-wmi-extras.ko.zst\n":    true,
		"/lib/modules/7.2.9-201.fc44.x86_64/extra/acerctl/acer-wmi-extras.ko.xz": false,
	} {
		if isDKMSPath(p) != want {
			t.Errorf("%s: want %v", p, want)
		}
	}
}

func TestWhyNotOnDisk(t *testing.T) {
	Root = t.TempDir()
	defer func() { Root = "" }()
	mk := func(path, content string) {
		p := filepath.Join(Root, path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("/proc/sys/kernel/osrelease", "6.12.48+deb13-amd64\n")
	if got := whyNotOnDisk(); got != "missing" {
		t.Fatalf("no package: got %q", got)
	}
	mk("/usr/src/acerctl-0.1.6/dkms.conf", "")
	if got := whyNotOnDisk(); got != "noheaders" {
		t.Fatalf("package, no headers: got %q", got)
	}
	mk("/lib/modules/6.12.48+deb13-amd64/build/Makefile", "")
	if got := whyNotOnDisk(); got != "notbuilt" {
		t.Fatalf("package and headers: got %q", got)
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
