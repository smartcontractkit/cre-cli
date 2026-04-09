package simulate

import (
	"testing"
)

func TestRedactURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "masks last path segment",
			raw:  "https://rpc.example.com/v1/my-secret-key",
			want: "https://rpc.example.com/v1/***",
		},
		{
			name: "removes query params",
			raw:  "https://rpc.example.com/v1/key?token=secret",
			want: "https://rpc.example.com/v1/***",
		},
		{
			name: "single path segment masked",
			raw:  "https://rpc.example.com/key",
			want: "https://rpc.example.com/***",
		},
		{
			name: "no path",
			raw:  "https://rpc.example.com",
			want: "https://rpc.example.com",
		},
		{
			name: "invalid URL",
			raw:  "://bad",
			want: "***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactURL(tt.raw)
			if got != tt.want {
				t.Errorf("redactURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
