// Package rules holds the pure policy of the daemon: which thermal profiles
// are allowed on which power source, how the Nitro key cycles, and how the
// five Acer modes map onto the three power-profiles-daemon profiles.
// See docs/api-dbus.md §4 and §11.
package rules

import "nitrosense/daemon/internal/hw"

// TurboMinPercent is the battery charge required for performance (Turbo)
// on AC and, as an extra, balanced-performance on battery.
const TurboMinPercent = 40

// Allowed returns the profiles permitted now, in the order of supported, and
// the subset that only exists because of a rule beyond the original app
// (flagged to the GUI through Mode.Extras).
func Allowed(supported []string, p hw.Power) (allowed, extras []string) {
	for _, m := range supported {
		switch {
		case p.Source == "ac":
			switch m {
			case "quiet", "balanced", "balanced-performance":
				allowed = append(allowed, m)
			case "performance":
				if p.Percent >= TurboMinPercent {
					allowed = append(allowed, m)
				}
			}
		default: // battery
			switch m {
			case "balanced", "low-power":
				allowed = append(allowed, m)
			case "balanced-performance":
				if p.Percent >= TurboMinPercent {
					allowed = append(allowed, m)
					extras = append(extras, m)
				}
			}
		}
	}
	return allowed, extras
}

// Contains reports whether list has s.
func Contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// Next returns the profile after current in allowed (wrapping), or the first
// allowed profile when current is not allowed. Empty when nothing is allowed.
func Next(allowed []string, current string) string {
	if len(allowed) == 0 {
		return ""
	}
	for i, m := range allowed {
		if m == current {
			return allowed[(i+1)%len(allowed)]
		}
	}
	return allowed[0]
}

// Fallback is the profile to apply when the current one became forbidden:
// the remembered profile for the source if still allowed, else balanced,
// else the first allowed one.
func Fallback(allowed []string, remembered string) string {
	if Contains(allowed, remembered) {
		return remembered
	}
	if Contains(allowed, "balanced") {
		return "balanced"
	}
	if len(allowed) > 0 {
		return allowed[0]
	}
	return ""
}

// PPD profile names (net.hadess.PowerProfiles).
const (
	PPDPowerSaver  = "power-saver"
	PPDBalanced    = "balanced"
	PPDPerformance = "performance"
)

// NearestPPD maps an Acer mode onto the closest PPD profile (many to one).
func NearestPPD(mode string) string {
	switch mode {
	case "low-power", "quiet":
		return PPDPowerSaver
	case "balanced-performance", "performance":
		return PPDPerformance
	default:
		return PPDBalanced
	}
}

// FromPPD picks the best Acer mode for a PPD profile chosen elsewhere (GNOME
// menu), given the power source and what is allowed now.
func FromPPD(ppd string, p hw.Power, allowed []string) string {
	var prefs []string
	switch ppd {
	case PPDPowerSaver:
		if p.Source == "ac" {
			prefs = []string{"quiet", "balanced"}
		} else {
			prefs = []string{"low-power", "balanced"}
		}
	case PPDPerformance:
		prefs = []string{"performance", "balanced-performance", "balanced"}
	default:
		prefs = []string{"balanced"}
	}
	for _, m := range prefs {
		if Contains(allowed, m) {
			return m
		}
	}
	return Fallback(allowed, "")
}

// FromExternal resolves a profile written to platform_profile by a third
// party (not through the PPD) into what the daemon keeps.
func FromExternal(written string, p hw.Power, allowed []string) string {
	if Contains(allowed, written) {
		return written
	}
	return FromPPD(NearestPPD(written), p, allowed)
}
