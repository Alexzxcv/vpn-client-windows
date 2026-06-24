package updater

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, candidate string
		want               bool
	}{
		{"v0.1.0", "v0.2.0", true},
		{"v0.1.0", "v0.1.1", true},
		{"v0.1.0", "v1.0.0", true},
		{"0.1.0", "0.1.0", false},
		{"v0.2.0", "v0.1.0", false},
		{"v1.2.3", "v1.2.3", false},
		{"v1.2.3", "v1.2.4", true},
		{"dev", "v0.1.0", true},     // dev build sees any release as newer
		{"v0.1.0", "garbage", false}, // unparseable remote tag is never newer
		{"v0.1.0", "", false},
		{"v1.0", "v1.0.1", true}, // short current version
		{"v1.2.3-rc1", "v1.2.3", false},
		{"v1.2.3", "v1.2.4-rc1", true}, // prerelease suffix ignored
	}
	for _, c := range cases {
		if got := IsNewer(c.current, c.candidate); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.current, c.candidate, got, c.want)
		}
	}
}

func TestPickAsset(t *testing.T) {
	assets := []Asset{
		{Name: "vpnclient-v0.2.0-windows-amd64.zip"},
		{Name: "SAPN-VPN-Setup.exe"},
	}
	got, ok := pickAsset(assets)
	if !ok || got.Name != "SAPN-VPN-Setup.exe" {
		t.Fatalf("pickAsset preferred %q (ok=%v), want the .exe installer", got.Name, ok)
	}

	zipOnly := []Asset{{Name: "vpnclient.zip"}}
	got, ok = pickAsset(zipOnly)
	if !ok || got.Name != "vpnclient.zip" {
		t.Fatalf("pickAsset(zipOnly) = %q (ok=%v), want the zip", got.Name, ok)
	}

	if _, ok := pickAsset(nil); ok {
		t.Fatal("pickAsset(nil) should report no asset")
	}
}
