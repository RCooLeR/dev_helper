package platform

import "testing"

func TestHostsLineHasDomain(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		domain string
		want   bool
	}{
		{name: "exact ipv4", line: "127.0.0.1 app.local # devhelper", domain: "app.local", want: true},
		{name: "exact ipv6", line: "::1 app.local # devhelper", domain: "app.local", want: true},
		{name: "multiple domains", line: "127.0.0.1 app.local api.local", domain: "api.local", want: true},
		{name: "partial is not match", line: "127.0.0.1 myapp.local # devhelper", domain: "app.local", want: false},
		{name: "comment ignored", line: "127.0.0.1 other.local # app.local", domain: "app.local", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hostsLineHasDomain(tt.line, tt.domain); got != tt.want {
				t.Fatalf("hostsLineHasDomain(%q, %q) = %v, want %v", tt.line, tt.domain, got, tt.want)
			}
		})
	}
}
