package tui_test

import "testing"

// The golden screens are written with slashes; Windows shows the same path
// with backslashes. slashed makes the two equal, and touches nothing else.
func TestSlashedMakesAWindowsHomePathMatchTheGoldenOne(t *testing.T) {
	got := slashed(`saved in ~\squad-up\presets\solo-indie · \no\change │ ~/already/fine`)

	want := `saved in ~/squad-up/presets/solo-indie · \no\change │ ~/already/fine`
	if got != want {
		t.Errorf("slashed = %q, want %q", got, want)
	}
}
