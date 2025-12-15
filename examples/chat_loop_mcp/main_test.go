package main

import "testing"

func TestCapitalizeRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "user", want: "User"},
		{input: "assistant", want: "Assistant"},
		{input: "system", want: "System"},
		{input: "unknown", want: "unknown"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := capitalizeRole(tt.input)
			if got != tt.want {
				t.Fatalf("capitalizeRole(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
