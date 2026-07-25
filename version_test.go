package miosa

import "testing"

func TestReleaseVersion(t *testing.T) {
	if sdkVersion != "2.0.2" {
		t.Fatalf("sdkVersion = %q, want 2.0.2", sdkVersion)
	}
}
