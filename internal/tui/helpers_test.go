package tui

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{
			name:  "shorter than max",
			input: "hello",
			max:   10,
			want:  "hello",
		},
		{
			name:  "equal to max",
			input: "hello",
			max:   5,
			want:  "hello",
		},
		{
			name:  "longer than max",
			input: "hello world",
			max:   8,
			want:  "hello...",
		},
		{
			name:  "max less than 4",
			input: "hello",
			max:   3,
			want:  "hel",
		},
		{
			name:  "empty string",
			input: "",
			max:   10,
			want:  "",
		},
		{
			name:  "unicode bytes",
			input: "你好世界",
			max:   12, // 4 chars * 3 bytes = 12 bytes
			want:  "你好世界",
		},
		{
			name:  "unicode truncated",
			input: "你好世界",
			max:   6, // 2 chars * 3 bytes = 6 bytes, but len("你好世界") = 12 > 6
			want:  "你...", // truncate works on bytes: s[:3] = "你"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

func TestGetCwd(t *testing.T) {
	// GetCwd should return a non-empty path
	cwd := GetCwd()
	if cwd == "" {
		t.Error("GetCwd() returned empty string")
	}
}
