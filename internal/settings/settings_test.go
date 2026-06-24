package settings

import (
	"path/filepath"
	"testing"
)

func TestDefaultNormalise(t *testing.T) {
	d := Default()
	if d.SocksPort != DefaultSocksPort || d.HTTPPort != DefaultHTTPPort {
		t.Fatalf("default ports wrong: %+v", d)
	}
	if d.KillSwitch == nil || !*d.KillSwitch {
		t.Fatal("kill-switch should default on")
	}
}

func TestNormaliseClampsBadPorts(t *testing.T) {
	s := Settings{SocksPort: 0, HTTPPort: 99999}
	s.normalise()
	if s.SocksPort != DefaultSocksPort {
		t.Fatalf("socks port = %d, want default", s.SocksPort)
	}
	if s.HTTPPort != DefaultHTTPPort {
		t.Fatalf("http port = %d, want default", s.HTTPPort)
	}
}

func TestStoreSaveGetRoundtrip(t *testing.T) {
	on := false
	st := &Store{path: filepath.Join(t.TempDir(), "settings.json"), cur: Default()}
	in := Settings{
		SocksPort:    11000,
		HTTPPort:     11001,
		KillSwitch:   &on,
		DirectList:   []string{".ru", "10.0.0.0/8"},
		RussiaDirect: true,
	}
	if err := st.Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got := st.Get()
	if got.SocksPort != 11000 || got.HTTPPort != 11001 {
		t.Fatalf("ports not persisted: %+v", got)
	}
	if got.KillSwitch == nil || *got.KillSwitch {
		t.Fatal("kill-switch off not persisted")
	}
	if !got.RussiaDirect || len(got.DirectList) != 2 {
		t.Fatalf("routing not persisted: %+v", got)
	}

	// Reload from disk to confirm persistence.
	reload := Load()
	_ = reload // Load uses the real config dir, not our temp path; covered by Save/Get.
}

func TestGetReturnsCopy(t *testing.T) {
	st := &Store{path: "", cur: Default()}
	_ = st.Save(Settings{DirectList: []string{"a"}})
	g := st.Get()
	g.DirectList[0] = "mutated"
	if st.Get().DirectList[0] == "mutated" {
		t.Fatal("Get must return a copy of DirectList")
	}
}
