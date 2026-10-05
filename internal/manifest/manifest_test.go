package manifest

import "testing"

func TestValidatePresets(t *testing.T) {
	ok := &Manifest{
		Version:    1,
		Components: []Component{{ID: "zellij", Name: "Zellij", Kind: "terminal"}},
		Presets:    map[string][]string{"minimal": {"zellij"}},
	}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}

	bad := &Manifest{
		Version:    1,
		Components: []Component{{ID: "zellij", Name: "Zellij", Kind: "terminal"}},
		Presets:    map[string][]string{"broken": {"nope"}},
	}
	if err := bad.Validate(); err == nil {
		t.Fatal("preset referencing an unknown component should fail validation")
	}
}

func TestResolvePreset(t *testing.T) {
	m := &Manifest{
		Version:    1,
		Components: []Component{{ID: "zellij", Name: "Zellij", Kind: "terminal"}},
		Presets:    map[string][]string{"minimal": {"zellij"}},
	}
	ids, err := m.ResolvePreset("minimal")
	if err != nil || len(ids) != 1 || ids[0] != "zellij" {
		t.Fatalf("resolve = %v, %v", ids, err)
	}
	if _, err := m.ResolvePreset("nope"); err == nil {
		t.Fatal("unknown preset should return an error")
	}
}
