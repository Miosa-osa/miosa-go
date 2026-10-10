package miosa

import "testing"

func TestReleaseVersion(t *testing.T) {
	if sdkVersion != "2.1.0" {
		t.Fatalf("sdkVersion = %q, want 2.1.0", sdkVersion)
	}
}
