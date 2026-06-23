//go:build windows

package sysproxy

import "testing"

func TestMentionsAnyPort(t *testing.T) {
	tests := []struct {
		name        string
		proxyServer string
		ports       []int
		want        bool
	}{
		{
			name:        "ours http+socks",
			proxyServer: "http=127.0.0.1:10801;https=127.0.0.1:10801;socks=127.0.0.1:10800",
			ports:       []int{10800, 10801},
			want:        true,
		},
		{
			name:        "only socks port present",
			proxyServer: "socks=127.0.0.1:10800",
			ports:       []int{10800, 10801},
			want:        true,
		},
		{
			name:        "foreign proxy",
			proxyServer: "http=10.0.0.1:8080",
			ports:       []int{10800, 10801},
			want:        false,
		},
		{
			name:        "zero ports ignored",
			proxyServer: "http=127.0.0.1:0",
			ports:       []int{0},
			want:        false,
		},
		{
			name:        "empty",
			proxyServer: "",
			ports:       []int{10800, 10801},
			want:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mentionsAnyPort(tt.proxyServer, tt.ports); got != tt.want {
				t.Fatalf("mentionsAnyPort(%q, %v) = %v, want %v", tt.proxyServer, tt.ports, got, tt.want)
			}
		})
	}
}
