package routing

import "testing"

func TestSplitList(t *testing.T) {
	domains, ips := SplitList([]string{
		" .ru ", "example.com", "10.0.0.0/8", "1.2.3.4", "", "  ", "СAPS.RU",
	})
	wantDomains := map[string]bool{".ru": true, "example.com": true, "сaps.ru": true}
	if len(domains) != 3 {
		t.Fatalf("domains = %v, want 3", domains)
	}
	for _, d := range domains {
		if !wantDomains[d] {
			t.Fatalf("unexpected domain %q in %v", d, domains)
		}
	}
	if len(ips) != 2 || ips[0] != "10.0.0.0/8" || ips[1] != "1.2.3.4" {
		t.Fatalf("ips = %v, want [10.0.0.0/8 1.2.3.4]", ips)
	}
}

func TestEmpty(t *testing.T) {
	if !(Options{}).Empty() {
		t.Fatal("zero Options should be empty")
	}
	if (Options{RussiaDirect: true}).Empty() {
		t.Fatal("RussiaDirect Options should not be empty")
	}
	if (Options{Domains: []string{"x"}}).Empty() {
		t.Fatal("Options with domains should not be empty")
	}
}
