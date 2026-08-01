package version

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "release", in: "1.2.3", want: "1.2.3"},
		{name: "tagged release", in: "v1.2.3", want: "1.2.3"},
		{name: "trimmed", in: "  v0.2.0  ", want: "0.2.0"},
		{name: "empty", in: "", want: "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Normalize(tt.in); got != tt.want {
				t.Fatalf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
