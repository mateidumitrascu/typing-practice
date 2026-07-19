package theme

import (
	"regexp"
	"testing"
)

func TestRegistry(t *testing.T) {
	all := All()
	if len(all) != 4 {
		t.Fatalf("expected 4 themes, got %d", len(all))
	}
	hex := regexp.MustCompile(`^#[0-9a-f]{6}$`)
	seen := map[string]bool{}
	for _, th := range all {
		if seen[th.Name] {
			t.Errorf("duplicate theme name %q", th.Name)
		}
		seen[th.Name] = true
		got, ok := Get(th.Name)
		if !ok || got.Name != th.Name {
			t.Errorf("Get(%q) failed", th.Name)
		}
		for _, c := range []string{
			th.Colors.Bg, th.Colors.Surface, th.Colors.Text,
			th.Colors.Subtext, th.Colors.Accent, th.Colors.Error, th.Colors.Success,
		} {
			if !hex.MatchString(c) {
				t.Errorf("theme %q has invalid color %q", th.Name, c)
			}
		}
	}
	if !seen[Default().Name] {
		t.Error("default theme not in registry")
	}
	if _, ok := Get("nope"); ok {
		t.Error("Get of unknown theme should fail")
	}
}
