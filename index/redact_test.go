package index

import "testing"

func TestRedactCommandParamExp(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple var", "echo $SECRET", "echo $REDACTED"},
		{"braced var", "echo ${SECRET}", "echo ${REDACTED}"},
		{"safe var HOME", "cd $HOME", "cd $HOME"},
		{"safe var PATH", "echo $PATH", "echo $PATH"},
		{"safe var PWD", "ls $PWD", "ls $PWD"},
		{"safe var USER", "echo $USER", "echo $USER"},
		{"special param $?", "echo $?", "echo $?"},
		{"special param $!", "echo $!", "echo $!"},
		{"special param $#", "echo $#", "echo $#"},
		{"special param $@", "echo $@", "echo $@"},
		{"special param $0", "echo $0", "echo $0"},
		{"special param $1", "echo $1", "echo $1"},
		{"special param $_", "echo $_", "echo $_"},
		{"mixed safe and sensitive", "curl -H $AUTH_TOKEN $HOME/file", "curl -H $REDACTED $HOME/file"},
		{"multiple sensitive", "echo $FOO $BAR", "echo $REDACTED $REDACTED"},
		{"no vars", "ls -la", "ls -la"},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactCommand(tt.input)
			if got != tt.want {
				t.Errorf("RedactCommand(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRedactCommandAssignment(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple assignment", "SECRET=hunter2 cmd", "SECRET=*** cmd"},
		{"export assignment", "export API_KEY=abc123", "export API_KEY=***"},
		{"safe var assignment", "HOME=/home/user cmd", "HOME=/home/user cmd"},
		{"PATH assignment", "PATH=/usr/bin cmd", "PATH=/usr/bin cmd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactCommand(tt.input)
			if got != tt.want {
				t.Errorf("RedactCommand(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRedactCommandSingleQuotes(t *testing.T) {
	// Single-quoted strings should not have their contents redacted
	// since $VAR inside single quotes is a literal, not an expansion.
	got := RedactCommand("echo '$SECRET'")
	if got != "echo '$SECRET'" {
		t.Errorf("single-quoted var should be preserved, got %q", got)
	}
}

func TestRedactCommandDoubleQuotes(t *testing.T) {
	got := RedactCommand(`echo "$SECRET"`)
	want := `echo "$REDACTED"`
	if got != want {
		t.Errorf("RedactCommand(echo \"$SECRET\") = %q, want %q", got, want)
	}
}

func TestRedactCommands(t *testing.T) {
	input := []string{
		"echo $SECRET",
		"ls -la",
		"export KEY=val",
	}
	got := RedactCommands(input)
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got))
	}
	if got[0] != "echo $REDACTED" {
		t.Errorf("got[0] = %q, want %q", got[0], "echo $REDACTED")
	}
	if got[1] != "ls -la" {
		t.Errorf("got[1] = %q, want %q", got[1], "ls -la")
	}
	if got[2] != "export KEY=***" {
		t.Errorf("got[2] = %q, want %q", got[2], "export KEY=***")
	}
}

func TestFilterQuoteContent(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`git commit -m "hello world"`, `git commit -m ""`},
		{`echo "[INIT] initialized" > demo.log`, `echo "" > demo.log`},
		{`node -e 'console.log("hello world!")'`, `node -e ''`},
		{`git status`, `git status`},
		{`echo "escaped \" quote"`, `echo ""`},
		{`python -c 'print(1+2)'`, `python -c ''`},
		{`grep "foo" bar.txt | wc -l`, `grep "" bar.txt | wc -l`},
		{`echo ""`, `echo ""`},
		{`echo ''`, `echo ''`},
		{`ls -la`, `ls -la`},
	}
	for _, tt := range tests {
		got := FilterQuoteContent(tt.input)
		if got != tt.want {
			t.Errorf("FilterQuoteContent(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFilterQuoteContentSliceDedup(t *testing.T) {
	cmds := []string{
		`git commit -m "fix: bug A"`,
		`git commit -m "feat: feature B"`,
		`git status`,
		`echo "hello"`,
		`echo "world"`,
	}
	got := FilterQuoteContentSlice(cmds)
	want := []string{`git commit -m ""`, `git status`, `echo ""`}
	if len(got) != len(want) {
		t.Fatalf("FilterQuoteContentSlice: got %d items %v, want %d items %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("FilterQuoteContentSlice[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRegexRedactFallback(t *testing.T) {
	// Test the regex fallback directly
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"brace var", "echo ${SECRET}", "echo ${REDACTED}"},
		{"simple var", "echo $SECRET", "echo $REDACTED"},
		{"safe brace var", "echo ${HOME}", "echo ${HOME}"},
		{"safe simple var", "echo $HOME", "echo $HOME"},
		{"assignment", "SECRET=val", "SECRET=***"},
		{"safe assignment", "HOME=/home/user", "HOME=/home/user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := regexRedact(tt.input)
			if got != tt.want {
				t.Errorf("regexRedact(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
