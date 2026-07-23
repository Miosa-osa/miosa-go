package miosa

import "testing"

func TestReleaseVersion(t *testing.T) {
	if sdkVersion != "2.0.1" {
		t.Fatalf("sdkVersion = %q, want 2.0.1", sdkVersion)
	}
}
