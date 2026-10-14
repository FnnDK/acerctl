package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMovesLegacyNitroKey(t *testing.T) {
	for _, tc := range []struct{ file, want string }{
		{`{"nitro_key":"app"}`, "app"},
		{`{"nitro_key":"app","sense_key":"cycle"}`, "cycle"},
		{`{"sense_key":"app"}`, "app"},
	} {
		p := filepath.Join(t.TempDir(), "state.json")
		if err := os.WriteFile(p, []byte(tc.file), 0o644); err != nil {
			t.Fatal(err)
		}
		d := Load(p).Data
		if d.SenseKey != tc.want || d.LegacyNitroKey != "" {
			t.Errorf("%s: SenseKey=%q LegacyNitroKey=%q, want %q and empty", tc.file, d.SenseKey, d.LegacyNitroKey, tc.want)
		}
	}
}
