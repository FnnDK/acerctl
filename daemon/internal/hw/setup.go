package hw

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// SetupItem is one change to the system that the app can make, or explain,
// for the user (System.Setup, docs/api-dbus.md §2). Every state is read from
// the system itself, so it survives daemon restarts.
type SetupItem struct {
	ID string // acer_wmi, signing_key or modules
	// State: "needed" (ApplySetup does it), "reboot" (done, takes effect at
	// the next boot) or "manual" (the user has to act; Detail says why).
	State  string
	Detail string
}

// Setup finds and applies the system changes. Both take effect at the next
// boot: the acer_wmi option when the module loads, the key in the firmware's
// enrollment screen.
type Setup interface {
	Items() []SetupItem
	// Apply makes one change. For signing_key it returns the one-time code
	// the firmware asks for at the next boot.
	Apply(id string) (code string, err error)
}

const (
	setupModprobe = "/etc/modprobe.d/acerctl.conf"
	scriptModules = "/var/lib/acer-wmi-modules" // packaging/install-modules.sh
)

// The public keys that sign the modules, by how they were built: akmods
// (Fedora; akmods-keys copies it for image-based systems) or DKMS (Debian's
// own key; Ubuntu signs with the shim-signed MOK).
var (
	akmodsCerts = []string{"/etc/pki/akmods/certs/public_key.der", "/etc/pki/akmods-keys/certs/public_key.der"}
	dkmsCerts   = []string{"/var/lib/dkms/mok.pub", "/var/lib/shim-signed/mok/MOK.der"}
)

// acerWMIGUIDs are the Acer WMI interfaces the companion modules use; one of
// them present means the modules belong on this machine.
var acerWMIGUIDs = []string{
	"61EF69EA-865C-4BC3-A502-A0DEBA0CB531", // APGe
	"7A4DDFE7-5B5D-40B4-8595-4408E0CC7F56", // gaming
	"79772EC5-04B1-4BFD-843C-61E7F77B6CC9", // battery
}

// validatedCaps are acer_wmi force_caps values checked on the real machine
// for models the kernel's quirk table does not list yet (the upstream patch
// of STATUS, backlog 1): they add fan control (pwm) to predator_v4.
var validatedCaps = map[string]string{"Nitro ANV15-52": "0x1C26"}

type realSetup struct{}

func (realSetup) Items() []SetupItem {
	var items []SetupItem
	if it, ok := acerWMIItem(); ok {
		items = append(items, it)
	}
	if it, ok := modulesItem(); ok {
		items = append(items, it)
	}
	return items
}

func (realSetup) Apply(id string) (string, error) {
	switch id {
	case "acer_wmi":
		return "", writeAcerWMIOptions(acerWMIOptions())
	case "signing_key":
		return enrollSigningKey()
	}
	return "", fmt.Errorf("nothing to apply for %q", id)
}

// ---- acer_wmi --------------------------------------------------------------

func acerWMIOptions() string {
	opts := "predator_v4=1"
	if p, err := readStr("/sys/class/dmi/id/product_name"); err == nil {
		if caps, ok := validatedCaps[p]; ok {
			opts += " force_caps=" + caps
		}
	}
	return opts
}

// acerWMIItem: the firmware reports thermal profiles (acer-wmi-extras reads
// its profile mask) but acer_wmi does not expose them, because predator_v4
// is off for a model its quirk table does not know. Older kernels have no
// predator_v4 at all (the parameter is missing from sysfs): then only a newer
// kernel helps, and writing the option would do nothing.
func acerWMIItem() (SetupItem, bool) {
	// supported_profiles is a WMI call: the free checks go first
	if !exists("/sys/module/acer_wmi") || hasPlatformProfile() {
		return SetupItem{}, false
	}
	if sp, err := readStr(filepath.Join(extrasDir, "supported_profiles")); err != nil || sp == "" {
		return SetupItem{}, false
	}
	if !exists("/sys/module/acer_wmi/parameters/predator_v4") {
		return SetupItem{ID: "acer_wmi", State: "manual", Detail: "oldkernel"}, true
	}
	it := SetupItem{ID: "acer_wmi", State: "needed", Detail: acerWMIOptions()}
	if modprobeHasPredatorV4() {
		it.State = "reboot"
	}
	return it, true
}

func hasPlatformProfile() bool {
	return len(glob(ppClassGlob)) > 0 || exists(ppLegacy)
}

// modprobeHasPredatorV4 reports an "options acer_wmi ... predator_v4=1" line
// in any modprobe.d file: written already, waiting for the module to reload.
func modprobeHasPredatorV4() bool { return predatorV4In("/etc/modprobe.d") }

func predatorV4In(dir string) bool {
	for _, f := range glob(filepath.Join(dir, "*.conf")) {
		b, err := readFile(f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 3 && fields[0] == "options" && fields[1] == "acer_wmi" && strings.Contains(line, "predator_v4=1") {
				return true
			}
		}
	}
	return false
}

func writeAcerWMIOptions(opts string) error {
	content := "# Written by acerd (acerctl setup): thermal profiles and fans from the firmware.\noptions acer_wmi " + opts + "\n"
	if DryRun {
		slog.Info("dry-run: skip write", "path", setupModprobe, "value", strings.TrimSpace(content))
		return nil
	}
	return os.WriteFile(rooted(setupModprobe), []byte(content), 0o644)
}

// ---- modules and the signing key -------------------------------------------

// modulesItem explains why acer-wmi-extras is not loaded on a machine with
// Acer WMI: not installed or not built, or refused by Secure Boot because the
// signing key is not enrolled (the one case ApplySetup can fix).
func modulesItem() (SetupItem, bool) {
	if exists("/sys/module/acer_wmi_extras") || !hasAcerWMI() {
		return SetupItem{}, false
	}
	if !moduleOnDisk("acer_wmi_extras") {
		return SetupItem{ID: "modules", State: "manual", Detail: whyNotOnDisk()}, true
	}
	if sigRequired() {
		switch ks := keyState(); ks {
		case "notenrolled":
			return SetupItem{ID: "signing_key", State: "needed"}, true
		case "pending":
			return SetupItem{ID: "signing_key", State: "reboot"}, true
		case "nokey", "nomokutil":
			return SetupItem{ID: "signing_key", State: "manual", Detail: ks}, true
		}
	}
	return SetupItem{ID: "modules", State: "manual", Detail: "notloaded"}, true
}

func hasAcerWMI() bool {
	for _, g := range acerWMIGUIDs {
		if len(glob("/sys/bus/wmi/devices/"+g+"*")) > 0 {
			return true
		}
	}
	return false
}

func moduleOnDisk(name string) bool {
	if Root == "" { // modinfo looks at the running system, not a fixture tree
		if err := exec.Command("modinfo", "-n", name).Run(); err == nil {
			return true
		}
	}
	return exists(filepath.Join(scriptModules, strings.ReplaceAll(name, "_", "-")+".ko"))
}

// whyNotOnDisk tells a missing modules package from an installed acerctl-dkms
// that has no build for the running kernel: Debian's dkms does not depend on
// the kernel headers and, without them, skips the build with only a warning
// (noheaders); with them, the build failed (notbuilt).
func whyNotOnDisk() string {
	if len(glob("/usr/src/acerctl-*/dkms.conf")) == 0 {
		return "missing"
	}
	if rel, err := readStr("/proc/sys/kernel/osrelease"); err == nil && !exists(filepath.Join("/lib/modules", rel, "build")) {
		return "noheaders"
	}
	return "notbuilt"
}

// sigRequired: the kernel only loads signed modules (Secure Boot lockdown).
func sigRequired() bool {
	if v, _ := readStr("/sys/module/module/parameters/sig_enforce"); v == "Y" {
		return true
	}
	l, _ := readStr("/sys/kernel/security/lockdown")
	return strings.Contains(l, "[integrity]") || strings.Contains(l, "[confidentiality]")
}

// signingCert is the public key the modules are signed with: DKMS installs
// them under updates/dkms, akmods and the script install elsewhere.
func signingCert() string {
	certs := akmodsCerts
	if Root != "" {
		// no modinfo for a fixture tree: the key it has decides
		if exists(dkmsCerts[0]) || exists(dkmsCerts[1]) {
			certs = dkmsCerts
		}
	} else if out, err := exec.Command("modinfo", "-n", "acer_wmi_extras").Output(); err == nil && isDKMSPath(string(out)) {
		certs = dkmsCerts
	}
	for _, c := range certs {
		if exists(c) {
			return c
		}
	}
	return ""
}

func isDKMSPath(p string) bool { return strings.Contains(p, "/updates/dkms/") }

// keyState of the signing key in the firmware's MOK list: enrolled,
// notenrolled, pending (enrollment requested, waiting for the next boot),
// nokey or nomokutil.
func keyState() string {
	cert := signingCert()
	if cert == "" {
		return "nokey"
	}
	if Root != "" { // mokutil reads the firmware, which a fixture tree does not have
		return "nomokutil"
	}
	if _, err := exec.LookPath("mokutil"); err != nil {
		return "nomokutil"
	}
	out, err := exec.Command("mokutil", "--test-key", cert).CombinedOutput()
	st := parseTestKey(string(out))
	if st == "" {
		slog.Warn("setup: unexpected mokutil answer", "out", strings.TrimSpace(string(out)), "err", err)
		return "notenrolled"
	}
	return st
}

// parseTestKey reads mokutil --test-key; its exit status differs between
// versions, the text does not.
func parseTestKey(out string) string {
	switch {
	case strings.Contains(out, "enrollment request"):
		return "pending"
	case strings.Contains(out, "already enrolled"):
		return "enrolled"
	case strings.Contains(out, "not enrolled"):
		return "notenrolled"
	}
	return ""
}

// enrollSigningKey requests the enrollment of the key akmods or DKMS already
// signs the modules with (no key is ever created here) with a fresh one-time
// code; a pending request is replaced. The firmware asks for the code at the
// next boot.
func enrollSigningKey() (string, error) {
	cert := signingCert()
	if cert == "" {
		return "", fmt.Errorf("no module signing key (%s, %s)", strings.Join(akmodsCerts, ", "), strings.Join(dkmsCerts, ", "))
	}
	code, err := oneTimeCode()
	if err != nil {
		return "", err
	}
	if Root != "" {
		return "", fmt.Errorf("no firmware to enroll %s in under %s", cert, Root)
	}
	if DryRun {
		slog.Info("dry-run: skip mokutil --import", "cert", cert)
		return code, nil
	}
	if keyState() == "pending" {
		if out, err := exec.Command("mokutil", "--revoke-import").CombinedOutput(); err != nil {
			return "", fmt.Errorf("mokutil --revoke-import: %v: %s", err, strings.TrimSpace(string(out)))
		}
	}
	hash, err := exec.Command("mokutil", "--generate-hash="+code).Output()
	if err != nil {
		return "", fmt.Errorf("mokutil --generate-hash: %w", err)
	}
	f, err := os.CreateTemp("", "acerd-mok-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(hash); err != nil {
		f.Close()
		return "", err
	}
	f.Close()
	if out, err := exec.Command("mokutil", "--import", cert, "--hash-file", f.Name()).CombinedOutput(); err != nil {
		return "", fmt.Errorf("mokutil --import: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return code, nil
}

// oneTimeCode is two digits, 10-99. The code is not a secret: typing it on
// the firmware's screen proves someone at the machine confirms the request,
// which no program can do, so short is fine (the screen accepts from one
// character). Digits only, as that screen uses a US keyboard map.
func oneTimeCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(90))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", 10+n.Int64()), nil
}

// ---- simulated -------------------------------------------------------------

type simSetup struct {
	mu    sync.Mutex
	items []SetupItem
}

// SimSetup pretends both changes are needed (acerd --simulate-setup), for
// developing the app's setup screen.
func SimSetup() Setup {
	return &simSetup{items: []SetupItem{
		{ID: "acer_wmi", State: "needed", Detail: "predator_v4=1"},
		{ID: "signing_key", State: "needed"},
	}}
}

func (s *simSetup) Items() []SetupItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]SetupItem(nil), s.items...)
}

func (s *simSetup) Apply(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == id {
			s.items[i].State = "reboot"
			if id == "signing_key" {
				return oneTimeCode()
			}
			return "", nil
		}
	}
	return "", fmt.Errorf("nothing to apply for %q", id)
}
