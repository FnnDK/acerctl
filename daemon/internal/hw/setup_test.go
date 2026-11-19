package hw

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	if len(s.Items()) != 3 {
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

func TestBlacklistItem(t *testing.T) {
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
	item := func() SetupItem {
		it, _ := blacklistItem()
		return it
	}
	damx := "/etc/modprobe.d/blacklist-acer_wmi.conf"
	mk(damx, "blacklist acer_wmi\n")
	if it := item(); it.ID != "" {
		t.Fatalf("no Acer WMI device, got %+v", it)
	}
	mk("/sys/bus/wmi/devices/7A4DDFE7-5B5D-40B4-8595-4408E0CC7F56-8/modalias", "wmi:7A4DDFE7-5B5D-40B4-8595-4408E0CC7F56\n")
	mk("/proc/cmdline", "quiet splash root=x\n")
	mk("/etc/modprobe.d/other.conf", "# blacklist acer_wmi\ninstall acer_wmi /sbin/modprobe --ignore-install acer_wmi\noptions snd_hda_intel power_save=1\n")
	if it := item(); it.State != "ask" || it.Detail != damx {
		t.Fatalf("DAMX file: got %+v", it)
	}

	// a file of the same name in /etc hides the package's one
	mk("/usr/lib/modprobe.d/blacklist-acer_wmi.conf", "blacklist acer_wmi\n")
	if it := item(); it.State != "ask" || it.Detail != damx {
		t.Fatalf("hidden package file: got %+v", it)
	}
	mk("/usr/lib/modprobe.d/tool.conf", "install acer-wmi /bin/false\n")
	if it := item(); it.State != "manual" || it.Detail != damx+" /usr/lib/modprobe.d/tool.conf" {
		t.Fatalf("package file: got %+v", it)
	}
	if err := unblockAcerWMI(); err == nil {
		t.Fatal("unblocked although a package file still blocks acer_wmi")
	}
	os.Remove(filepath.Join(Root, "/usr/lib/modprobe.d/tool.conf"))
	mk("/proc/cmdline", "quiet modprobe.blacklist=nouveau,acer_wmi\n")
	if it := item(); it.State != "manual" || it.Detail != damx+" modprobe.blacklist=nouveau,acer_wmi" {
		t.Fatalf("command line: got %+v", it)
	}
	mk("/proc/cmdline", "quiet splash\n")

	// the DAMX file goes; elsewhere only the blocking line becomes a comment
	mk("/etc/modprobe.d/mixed.conf", "options acer_wmi force_caps=0x1C26\nblacklist acer_wmi\n")
	if err := unblockAcerWMI(); err != nil {
		t.Fatal(err)
	}
	// it hides the package's file of the same name, so it stays, commented
	if b, _ := readFile(damx); string(b) != "# Disabled by acerd (acerctl setup) so that acer_wmi loads: blacklist acer_wmi\n" {
		t.Fatalf("%s:\n%s", damx, b)
	}
	b, _ := readFile("/etc/modprobe.d/mixed.conf")
	want := "options acer_wmi force_caps=0x1C26\n# Disabled by acerd (acerctl setup) so that acer_wmi loads: blacklist acer_wmi\n"
	if string(b) != want {
		t.Fatalf("mixed.conf:\n%s", b)
	}
	if b, _ := readFile("/etc/modprobe.d/other.conf"); !strings.Contains(string(b), "\ninstall acer_wmi /sbin/modprobe") {
		t.Fatalf("other.conf changed:\n%s", b)
	}
	if it := item(); it.ID != "" { // under a fixture tree there is no reboot state
		t.Fatalf("after unblocking: got %+v", it)
	}

	// with nothing to hide, a blacklist-only file goes
	os.Remove(filepath.Join(Root, "/usr/lib/modprobe.d/blacklist-acer_wmi.conf"))
	mk(damx, "blacklist acer_wmi\n")
	if err := unblockAcerWMI(); err != nil {
		t.Fatal(err)
	}
	if exists(damx) {
		t.Fatal("the blacklist-only file is still there")
	}
}
