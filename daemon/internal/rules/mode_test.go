package rules

import (
	"reflect"
	"testing"

	"acerctl/daemon/internal/hw"
)

var anv15 = []string{"low-power", "quiet", "balanced", "balanced-performance"}
var withTurbo = []string{"low-power", "quiet", "balanced", "balanced-performance", "performance"}

func TestAllowed(t *testing.T) {
	cases := []struct {
		name      string
		supported []string
		p         hw.Power
		allowed   []string
		extras    []string
	}{
		{"ac no turbo", anv15, hw.Power{Source: "ac", Percent: 90}, []string{"quiet", "balanced", "balanced-performance"}, nil},
		{"ac turbo full", withTurbo, hw.Power{Source: "ac", Percent: 90}, []string{"quiet", "balanced", "balanced-performance", "performance"}, nil},
		{"ac turbo low", withTurbo, hw.Power{Source: "ac", Percent: 39}, []string{"quiet", "balanced", "balanced-performance"}, nil},
		{"battery high", anv15, hw.Power{Source: "battery", Percent: 80}, []string{"low-power", "balanced", "balanced-performance"}, []string{"balanced-performance"}},
		{"battery low", anv15, hw.Power{Source: "battery", Percent: 30}, []string{"low-power", "balanced"}, nil},
	}
	for _, c := range cases {
		a, e := Allowed(c.supported, c.p)
		if !reflect.DeepEqual(a, c.allowed) || !reflect.DeepEqual(e, c.extras) {
			t.Errorf("%s: got %v %v, want %v %v", c.name, a, e, c.allowed, c.extras)
		}
	}
}

func TestNext(t *testing.T) {
	a := []string{"quiet", "balanced", "balanced-performance"}
	if got := Next(a, "balanced-performance"); got != "quiet" {
		t.Errorf("wrap: %s", got)
	}
	if got := Next(a, "low-power"); got != "quiet" {
		t.Errorf("not allowed: %s", got)
	}
}

func TestPPD(t *testing.T) {
	ac := hw.Power{Source: "ac", Percent: 90}
	bat := hw.Power{Source: "battery", Percent: 90}
	batLow := hw.Power{Source: "battery", Percent: 20}
	cases := []struct {
		ppd  string
		p    hw.Power
		sup  []string
		want string
	}{
		{PPDPowerSaver, ac, anv15, "quiet"},
		{PPDPowerSaver, bat, anv15, "low-power"},
		{PPDPerformance, ac, anv15, "balanced-performance"},
		{PPDPerformance, ac, withTurbo, "performance"},
		{PPDPerformance, bat, anv15, "balanced-performance"},
		{PPDPerformance, batLow, anv15, "balanced"},
		{PPDBalanced, bat, anv15, "balanced"},
	}
	for _, c := range cases {
		allowed, _ := Allowed(c.sup, c.p)
		if got := FromPPD(c.ppd, c.p, allowed); got != c.want {
			t.Errorf("FromPPD(%s, %v) = %s, want %s", c.ppd, c.p, got, c.want)
		}
	}
	for mode, want := range map[string]string{"low-power": PPDPowerSaver, "quiet": PPDPowerSaver,
		"balanced": PPDBalanced, "balanced-performance": PPDPerformance, "performance": PPDPerformance} {
		if got := NearestPPD(mode); got != want {
			t.Errorf("NearestPPD(%s) = %s", mode, got)
		}
	}
	// A third party writing low-power on AC becomes quiet.
	allowed, _ := Allowed(anv15, ac)
	if got := FromExternal("low-power", ac, allowed); got != "quiet" {
		t.Errorf("FromExternal low-power on ac = %s", got)
	}
}
