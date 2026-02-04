package main

import "testing"

func TestEscapeArg(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple flag",
			input: "-c",
			want:  "-c",
		},
		{
			name:  "long flag",
			input: "--dangerously-skip-permissions",
			want:  "--dangerously-skip-permissions",
		},
		{
			name:  "flag with value no space",
			input: "--model=gpt-4",
			want:  "--model=gpt-4",
		},
		{
			name:  "value with space",
			input: "hello world",
			want:  "$'hello world'",
		},
		{
			name:  "value with single quote",
			input: "it's",
			want:  "$'it\\'s'",
		},
		{
			name:  "value with double quote",
			input: `say "hello"`,
			want:  `$'say "hello"'`,
		},
		{
			name:  "value with dollar",
			input: "$HOME/path",
			want:  "$'$HOME/path'",
		},
		{
			name:  "value with backtick",
			input: "`whoami`",
			want:  "$'`whoami`'",
		},
		{
			name:  "value with semicolon",
			input: "a;b",
			want:  "$'a;b'",
		},
		{
			name:  "value with pipe",
			input: "a|b",
			want:  "$'a|b'",
		},
		{
			name:  "value with ampersand",
			input: "a&b",
			want:  "$'a&b'",
		},
		{
			name:  "value with glob",
			input: "*.txt",
			want:  "$'*.txt'",
		},
		{
			name:  "value with brackets",
			input: "[a-z]",
			want:  "$'[a-z]'",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeArg(tt.input)
			if got != tt.want {
				t.Errorf("escapeArg(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestJoinArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "single flag",
			args: []string{"-c"},
			want: "-c",
		},
		{
			name: "multiple flags",
			args: []string{"-c", "--verbose"},
			want: "-c --verbose",
		},
		{
			name: "flag with value containing space",
			args: []string{"-m", "hello world"},
			want: "-m $'hello world'",
		},
		{
			name: "empty args",
			args: []string{},
			want: "",
		},
		{
			name: "mixed safe and unsafe",
			args: []string{"--safe", "unsafe;value", "-x"},
			want: "--safe $'unsafe;value' -x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinArgs(tt.args)
			if got != tt.want {
				t.Errorf("joinArgs(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestNormalizeSessionName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"claude", "ai-claude"},
		{"ai-claude", "ai-claude"},
		{"codex", "ai-codex"},
		{"ai-codex-cli", "ai-codex-cli"},
	}

	for _, tt := range tests {
		got := normalizeSessionName(tt.input)
		if got != tt.want {
			t.Errorf("normalizeSessionName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
