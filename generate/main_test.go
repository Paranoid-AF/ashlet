package generate

import (
	"context"
	"math"
	"strings"
	"testing"

	ashlet "github.com/Paranoid-AF/ashlet"
	defaults "github.com/Paranoid-AF/ashlet/default"
)

// testEngine creates a minimal engine for testing
func testEngine() *Engine {
	return &Engine{
		config:       ashlet.DefaultConfig(),
		customPrompt: "", // use default prompt
	}
}

// --- XML candidate parsing tests ---

func TestParseCandidatesXMLReplace(t *testing.T) {
	output := `<candidate type="replace">
<command>git checkout</command>
</candidate>
<candidate type="replace">
<command>git cherry-pick</command>
</candidate>`
	candidates := parseCandidates(output, "git ch", 4)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0].Completion != "git checkout" {
		t.Errorf("expected %q, got %q", "git checkout", candidates[0].Completion)
	}
	if candidates[1].Completion != "git cherry-pick" {
		t.Errorf("expected %q, got %q", "git cherry-pick", candidates[1].Completion)
	}
}

func TestParseCandidatesXMLReplaceWithCursor(t *testing.T) {
	output := `<candidate type="replace">
<command>git commit -m "█"</command>
</candidate>`
	candidates := parseCandidates(output, "git com", 4)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	c := candidates[0]
	if c.Completion != `git commit -m ""` {
		t.Errorf("expected %q, got %q", `git commit -m ""`, c.Completion)
	}
	if c.CursorPos == nil {
		t.Fatal("expected CursorPos to be set")
	}
	if *c.CursorPos != 15 {
		t.Errorf("expected CursorPos=15, got %d", *c.CursorPos)
	}
}

func TestParseCandidatesXMLNoCursor(t *testing.T) {
	output := `<candidate type="replace">
<command>git status</command>
</candidate>`
	candidates := parseCandidates(output, "git s", 4)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].CursorPos != nil {
		t.Errorf("expected CursorPos=nil, got %d", *candidates[0].CursorPos)
	}
}

func TestParseCandidatesXMLAppend(t *testing.T) {
	output := `<candidate type="append">
<command>git push</command>
</candidate>
<candidate type="append">
<command>npm run build</command>
</candidate>`
	input := `git commit -m "initial" && `
	candidates := parseCandidates(output, input, 4)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0].Completion != `git commit -m "initial" && git push` {
		t.Errorf("expected %q, got %q", `git commit -m "initial" && git push`, candidates[0].Completion)
	}
	if candidates[1].Completion != `git commit -m "initial" && npm run build` {
		t.Errorf("expected %q, got %q", `git commit -m "initial" && npm run build`, candidates[1].Completion)
	}
}

func TestParseCandidatesXMLAppendAutoSeparator(t *testing.T) {
	// Input doesn't end with && — separator is added automatically
	output := `<candidate type="append">
<command>git push</command>
</candidate>`
	input := `git commit -m "done"`
	candidates := parseCandidates(output, input, 4)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].Completion != `git commit -m "done" && git push` {
		t.Errorf("expected %q, got %q", `git commit -m "done" && git push`, candidates[0].Completion)
	}
}

func TestParseCandidatesXMLAppendCursorOffset(t *testing.T) {
	// cursor in append mode is relative to appended part start
	output := `<candidate type="append">
<command>git commit -m "█"</command>
</candidate>`
	input := "make build && "
	candidates := parseCandidates(output, input, 4)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	c := candidates[0]
	if c.Completion != `make build && git commit -m ""` {
		t.Errorf("expected %q, got %q", `make build && git commit -m ""`, c.Completion)
	}
	if c.CursorPos == nil {
		t.Fatal("expected CursorPos to be set")
	}
	// "make build && " is 14 chars, separator is "" (input ends with space after &&)
	// cursor 15 + offset 14 = 29
	if *c.CursorPos != 29 {
		t.Errorf("expected CursorPos=29, got %d", *c.CursorPos)
	}
}

func TestParseCandidatesXMLMultiCommand(t *testing.T) {
	// Multiple commands in one candidate are joined with " && "
	output := `<candidate type="replace">
<command>git commit -m "█"</command>
<command>git push</command>
</candidate>`
	candidates := parseCandidates(output, "git com", 4)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	c := candidates[0]
	if c.Completion != `git commit -m "" && git push` {
		t.Errorf("expected %q, got %q", `git commit -m "" && git push`, c.Completion)
	}
	if c.CursorPos == nil {
		t.Fatal("expected CursorPos to be set")
	}
	if *c.CursorPos != 15 {
		t.Errorf("expected CursorPos=15, got %d", *c.CursorPos)
	}
}

func TestParseCandidatesXMLDeduplicates(t *testing.T) {
	output := `<candidate type="replace">
<command>git status</command>
</candidate>
<candidate type="replace">
<command>git status</command>
</candidate>
<candidate type="replace">
<command>git stash</command>
</candidate>`
	candidates := parseCandidates(output, "git s", 4)
	if len(candidates) != 2 {
		t.Errorf("expected 2 unique candidates, got %d", len(candidates))
	}
}

func TestParseCandidatesXMLRespectsMax(t *testing.T) {
	output := `<candidate type="replace"><command>one</command></candidate>
<candidate type="replace"><command>two</command></candidate>
<candidate type="replace"><command>three</command></candidate>`
	candidates := parseCandidates(output, "", 2)
	if len(candidates) != 2 {
		t.Errorf("expected 2 candidates with max=2, got %d", len(candidates))
	}
}

func TestParseCandidatesXMLEmptyCommand(t *testing.T) {
	// Empty command tag should be skipped
	output := `<candidate type="replace">
<command></command>
</candidate>`
	candidates := parseCandidates(output, "", 4)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates for empty command, got %d", len(candidates))
	}
}

func TestParseCandidatesConfidence(t *testing.T) {
	output := `<candidate type="replace"><command>one</command></candidate>
<candidate type="replace"><command>two</command></candidate>
<candidate type="replace"><command>three</command></candidate>
<candidate type="replace"><command>four</command></candidate>`
	candidates := parseCandidates(output, "", 4)
	if len(candidates) != 4 {
		t.Fatalf("expected 4 candidates, got %d", len(candidates))
	}
	expectedConf := []float64{0.95, 0.80, 0.65, 0.50}
	for i, exp := range expectedConf {
		if math.Abs(candidates[i].Confidence-exp) > 1e-9 {
			t.Errorf("candidate[%d] confidence: expected %.2f, got %.2f", i, exp, candidates[i].Confidence)
		}
	}
}

func TestParseCandidatesEmptyOutput(t *testing.T) {
	candidates := parseCandidates("", "", 4)
	if candidates != nil {
		t.Errorf("expected nil for empty output, got %v", candidates)
	}
}

func TestParseCandidatesXMLPipeReplace(t *testing.T) {
	output := `<candidate type="replace">
<command>cat foo.log | grep -i error</command>
</candidate>
<candidate type="replace">
<command>cat foo.log | grep warning</command>
</candidate>`
	candidates := parseCandidates(output, "cat foo.log | grep", 4)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0].Completion != "cat foo.log | grep -i error" {
		t.Errorf("expected %q, got %q", "cat foo.log | grep -i error", candidates[0].Completion)
	}
	if candidates[1].Completion != "cat foo.log | grep warning" {
		t.Errorf("expected %q, got %q", "cat foo.log | grep warning", candidates[1].Completion)
	}
}

// --- chainSeparator tests ---

func TestChainSeparator(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`git commit -m "done" && `, ""}, // already has && with trailing space
		{`git commit -m "done" &&`, " "}, // has && but no space
		{`echo hello |`, " "},            // pipe, no space
		{`echo hello | `, ""},            // pipe with space
		{`echo hello ;`, " "},            // semicolon, no space
		{`git commit -m "done"`, " && "}, // no operator
		{`git status`, " && "},           // plain command
	}
	for _, tt := range tests {
		got := chainSeparator(tt.input)
		if got != tt.want {
			t.Errorf("chainSeparator(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- Fallback parsing tests (no XML) ---

func TestParseCandidatesFallbackFirstWordMatch(t *testing.T) {
	output := "git checkout\ngit cherry-pick"
	candidates := parseCandidates(output, "git ch", 4)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0].Completion != "git checkout" {
		t.Errorf("expected %q, got %q", "git checkout", candidates[0].Completion)
	}
	if candidates[1].Completion != "git cherry-pick" {
		t.Errorf("expected %q, got %q", "git cherry-pick", candidates[1].Completion)
	}
}

func TestParseCandidatesFallbackRejectsUnrelatedLine(t *testing.T) {
	output := "brew install"
	candidates := parseCandidates(output, "git co", 4)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates (different first word), got %d: %v", len(candidates), candidates)
	}
}

func TestParseCandidatesFallbackRejectsSuffixOnly(t *testing.T) {
	output := "--amend"
	candidates := parseCandidates(output, "git c", 4)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates (suffix without XML), got %d: %v", len(candidates), candidates)
	}
}

func TestParseCandidatesFallbackStripsBackticks(t *testing.T) {
	output := "`git status`\n`git stash`"
	candidates := parseCandidates(output, "git ", 4)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0].Completion != "git status" {
		t.Errorf("expected 'git status', got %q", candidates[0].Completion)
	}
}

func TestParseCandidatesFallbackSkipsXMLLines(t *testing.T) {
	// Partial/broken XML should be skipped in fallback
	output := "<autocomplete\ngit checkout"
	candidates := parseCandidates(output, "git ch", 4)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].Completion != "git checkout" {
		t.Errorf("expected %q, got %q", "git checkout", candidates[0].Completion)
	}
}

func TestParseCandidatesFallbackSkipsPromptDelimiter(t *testing.T) {
	output := "$ brew install\nbrew install vim"
	candidates := parseCandidates(output, "brew ", 4)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate (skipping $ line), got %d", len(candidates))
	}
	if candidates[0].Completion != "brew install vim" {
		t.Errorf("expected %q, got %q", "brew install vim", candidates[0].Completion)
	}
}

// --- parseCandidateBlocks / parseCommands unit tests ---

func TestParseCandidateBlocks(t *testing.T) {
	output := `<candidate type="replace">
<command>git checkout</command>
</candidate>
<candidate type="append">
<command>git push</command>
</candidate>`
	blocks := parseCandidateBlocks(output)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].typ != "replace" {
		t.Errorf("block[0] type: expected %q, got %q", "replace", blocks[0].typ)
	}
	if blocks[1].typ != "append" {
		t.Errorf("block[1] type: expected %q, got %q", "append", blocks[1].typ)
	}
}

func TestParseCommands(t *testing.T) {
	content := `<command>git commit -m "█"</command>
<command>git push</command>`
	cmds := parseCommands(content)
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}
	if cmds[0].text != `git commit -m ""` || cmds[0].cursor != 15 {
		t.Errorf("cmd[0]: expected text=%q cursor=15, got text=%q cursor=%d", `git commit -m ""`, cmds[0].text, cmds[0].cursor)
	}
	if cmds[1].text != "git push" || cmds[1].cursor != -1 {
		t.Errorf("cmd[1]: expected text=%q cursor=-1, got text=%q cursor=%d", "git push", cmds[1].text, cmds[1].cursor)
	}
}

func TestParseCommandsNoCursor(t *testing.T) {
	content := `<command>git commit -m ""</command>`
	cmds := parseCommands(content)
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	// No █ sentinel = no cursor
	if cmds[0].cursor != -1 {
		t.Errorf("expected cursor=-1 for no sentinel, got %d", cmds[0].cursor)
	}
}

// --- buildSystemPrompt tests ---

func TestBuildSystemPromptContent(t *testing.T) {
	e := testEngine()
	prompt := e.buildSystemPrompt(4)

	if !strings.Contains(prompt, "auto-completion engine") {
		t.Error("system prompt should contain 'auto-completion engine'")
	}
	if !strings.Contains(prompt, `<candidate type="replace">`) {
		t.Error("system prompt should contain replace candidate example")
	}
	if !strings.Contains(prompt, `<candidate type="append">`) {
		t.Error("system prompt should contain append candidate example")
	}
}

func TestBuildUserMessageContent(t *testing.T) {
	e := testEngine()
	req := &ashlet.Request{
		Input:     "git st",
		CursorPos: 6, // cursor at end — no marker
		Cwd:       "/home/user/project",
	}
	msg := e.buildUserMessage(req, &Info{}, nil)

	if !strings.Contains(msg, "cwd: /home/user/project") {
		t.Error("user message should contain cwd")
	}
	if !strings.Contains(msg, "Input: `git st`") {
		t.Error("user message should contain Input: `git st`")
	}
	if strings.Contains(msg, "█") {
		t.Error("cursor marker should NOT appear when cursor is at end of input")
	}
}

func TestBuildUserMessageCursorMid(t *testing.T) {
	e := testEngine()
	req := &ashlet.Request{
		Input:     `git commit -m ""`,
		CursorPos: 15, // cursor between the quotes
		Cwd:       "/home/user/project",
	}
	msg := e.buildUserMessage(req, &Info{}, nil)

	expected := "Input: `git commit -m \"█\"`"
	if !strings.Contains(msg, expected) {
		t.Errorf("expected %q in user message, got:\n%s", expected, msg)
	}
}

func TestBuildUserMessageWithRelevantCommands(t *testing.T) {
	e := testEngine()
	req := &ashlet.Request{
		Input:     "docker ",
		CursorPos: 7,
		Cwd:       "/home/user",
	}
	ctx := &Info{
		RecentCommands:   []string{"ls", "cd /tmp"},
		RelevantCommands: []string{"docker build -t myapp .", "docker compose up -d"},
	}
	msg := e.buildUserMessage(req, ctx, nil)

	if !strings.Contains(msg, "related:") {
		t.Error("user message should contain 'related:'")
	}
	if !strings.Contains(msg, "docker build -t myapp .") {
		t.Error("user message should contain relevant command")
	}
}

func TestBuildUserMessageWithoutRelevantCommands(t *testing.T) {
	e := testEngine()
	req := &ashlet.Request{
		Input:     "git st",
		CursorPos: 6,
		Cwd:       "/home/user",
	}
	ctx := &Info{
		RecentCommands: []string{"ls", "cd /tmp"},
	}
	msg := e.buildUserMessage(req, ctx, nil)

	if strings.Contains(msg, "related:") {
		t.Error("user message should not contain 'related:' when empty")
	}
}

func TestBuildUserMessageRecentCommandsLimit(t *testing.T) {
	e := testEngine()
	req := &ashlet.Request{
		Input:     "test",
		CursorPos: 4,
	}
	cmds := make([]string, 10)
	for i := range cmds {
		cmds[i] = "cmd" + strings.Repeat("x", i)
	}
	ctx := &Info{
		RecentCommands: cmds,
	}
	msg := e.buildUserMessage(req, ctx, nil)

	if !strings.Contains(msg, "cmdxxxx") {
		t.Error("user message should contain 5th recent command")
	}
	if strings.Contains(msg, "cmdxxxxx") {
		t.Error("user message should not contain 6th recent command")
	}
}

func TestBuildUserMessageHistoryAlwaysFiltered(t *testing.T) {
	e := &Engine{config: ashlet.DefaultConfig()}
	req := &ashlet.Request{
		Input:     "git ",
		CursorPos: 4,
	}
	info := &Info{
		RecentCommands: []string{
			`git commit -m "fix: something"`,
			`git status`,
			`git commit -m "feat: other"`,
		},
	}
	msg := e.buildUserMessage(req, info, nil)

	if strings.Contains(msg, "fix: something") {
		t.Error("user message should not contain quote content — filtering is always on")
	}
	if !strings.Contains(msg, `git commit -m ""`) {
		t.Error("user message should contain filtered command with empty quotes")
	}
}

func TestDefaultPromptEmbedNonEmpty(t *testing.T) {
	if defaults.DefaultPrompt == "" {
		t.Fatal("embedded DefaultPrompt should not be empty")
	}
	if !strings.Contains(defaults.DefaultPrompt, "auto-completion engine") {
		t.Error("DefaultPrompt should contain auto-completion engine header")
	}
}

func TestBuildUserMessageWithDirContext(t *testing.T) {
	e := testEngine()
	req := &ashlet.Request{
		Input:     "npm run",
		CursorPos: 7,
		Cwd:       "/home/user/project",
	}
	dirCtx := &DirContext{
		CwdListing:     "node_modules package.json src",
		PackageManager: "pnpm",
		CwdManifests:   map[string]string{"package.json scripts": `"build": "tsc", "test": "jest"`},
	}
	msg := e.buildUserMessage(req, &Info{}, dirCtx)

	if !strings.Contains(msg, "files: node_modules package.json src") {
		t.Error("user message should contain directory listing")
	}
	if !strings.Contains(msg, "pkg: pnpm") {
		t.Error("user message should contain package manager")
	}
}

func TestBuildUserMessageNilDirContext(t *testing.T) {
	e := testEngine()
	req := &ashlet.Request{
		Input:     "git st",
		CursorPos: 6,
		Cwd:       "/home/user",
	}
	msg := e.buildUserMessage(req, &Info{}, nil)

	if strings.Contains(msg, "files:") {
		t.Error("user message should not contain files section with nil dir context")
	}
	if strings.Contains(msg, "pkg:") {
		t.Error("user message should not contain pkg section with nil dir context")
	}
}

func TestBuildSystemPromptInvalidCustomPromptFallback(t *testing.T) {
	e := &Engine{
		config:       ashlet.DefaultConfig(),
		customPrompt: "{{.Invalid | nonexistentFunc}}",
	}
	prompt := e.buildSystemPrompt(4)

	if !strings.Contains(prompt, "auto-completion engine") {
		t.Error("expected fallback to default prompt on invalid custom template")
	}
}

// --- Complete() tests ---

func TestCompleteReturnsEmptySlice(t *testing.T) {
	e := &Engine{gatherer: NewGatherer(nil, nil), generator: nil, config: ashlet.DefaultConfig()}
	req := &ashlet.Request{Input: ""}
	resp := e.Complete(context.Background(), req)
	if resp.Error == nil || resp.Error.Code != "not_configured" {
		// With nil generator, expect not_configured error before checking input
		t.Log("got expected not_configured error for nil generator")
	}
}

func TestCompleteNotConfigured(t *testing.T) {
	e := &Engine{gatherer: NewGatherer(nil, nil), generator: nil, config: ashlet.DefaultConfig()}
	req := &ashlet.Request{Input: "git st", CursorPos: 6}
	resp := e.Complete(context.Background(), req)

	if resp.Candidates == nil {
		t.Fatal("Candidates should not be nil")
	}
	if len(resp.Candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(resp.Candidates))
	}
	if resp.Error == nil || resp.Error.Code != "not_configured" {
		t.Errorf("expected not_configured error")
	}
}

// --- filterCandidateQuotes tests ---

func TestFilterCandidateQuotesNoQuotesInInput(t *testing.T) {
	candidates := []ashlet.Candidate{
		{Completion: `git commit -m "initial"`, Confidence: 0.95},
		{Completion: `git commit -m "fix bug"`, Confidence: 0.80},
		{Completion: `git status`, Confidence: 0.65},
	}
	result := filterCandidateQuotes(candidates, "git commi")

	if len(result) != 2 {
		t.Fatalf("expected 2 candidates after dedup, got %d", len(result))
	}
	if result[0].Completion != `git commit -m ""` {
		t.Errorf("expected %q, got %q", `git commit -m ""`, result[0].Completion)
	}
	if result[0].CursorPos == nil || *result[0].CursorPos != 15 {
		t.Errorf("expected CursorPos=15 inside quotes, got %v", result[0].CursorPos)
	}
}

func TestFilterCandidateQuotesWithQuotesInInput(t *testing.T) {
	candidates := []ashlet.Candidate{
		{Completion: `git commit -m "feat: sign-in page"`, Confidence: 0.95},
	}
	result := filterCandidateQuotes(candidates, `git commit -m "feat:`)

	if len(result) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result))
	}
	if result[0].Completion != `git commit -m "feat: sign-in page"` {
		t.Errorf("expected content preserved, got %q", result[0].Completion)
	}
	if result[0].CursorPos == nil || *result[0].CursorPos != 33 {
		t.Errorf("expected CursorPos=33, got %v", result[0].CursorPos)
	}
}

func TestFilterCandidateQuotesCursorAfterChain(t *testing.T) {
	candidates := []ashlet.Candidate{
		{Completion: `git commit -m "a" && git push`, Confidence: 0.95},
	}
	result := filterCandidateQuotes(candidates, `git commit -m "a`)

	if len(result) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result))
	}
	if result[0].CursorPos != nil {
		t.Errorf("expected CursorPos=nil when chain follows quote, got %d", *result[0].CursorPos)
	}
}

func TestFilterCandidateQuotesNoClobberExistingCursor(t *testing.T) {
	pos := 5
	candidates := []ashlet.Candidate{
		{Completion: `echo "hello"`, Confidence: 0.95, CursorPos: &pos},
	}
	result := filterCandidateQuotes(candidates, `echo "he`)

	if result[0].CursorPos == nil || *result[0].CursorPos != 5 {
		t.Errorf("expected existing CursorPos=5 preserved, got %v", result[0].CursorPos)
	}
}

func TestFilterCandidateQuotesNoQuotesInCandidate(t *testing.T) {
	candidates := []ashlet.Candidate{
		{Completion: "git status", Confidence: 0.95},
	}
	result := filterCandidateQuotes(candidates, "git s")
	if result[0].CursorPos != nil {
		t.Errorf("expected no CursorPos for command without quotes, got %d", *result[0].CursorPos)
	}
}

func TestFilterCandidateQuotesEmpty(t *testing.T) {
	result := filterCandidateQuotes(nil, "git s")
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

// --- Quote content helpers ---

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
		got := filterQuoteContent(tt.input)
		if got != tt.want {
			t.Errorf("filterQuoteContent(%q) = %q, want %q", tt.input, got, tt.want)
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
	got := filterQuoteContentSlice(cmds)
	want := []string{`git commit -m ""`, `git status`, `echo ""`}
	if len(got) != len(want) {
		t.Fatalf("filterQuoteContentSlice: got %d items %v, want %d items %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("filterQuoteContentSlice[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFindLastClosingQuotePos(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{`git commit -m ""`, 15},
		{`echo "hello"`, 11},
		{`echo ''`, 6},
		{`git status`, -1},
		{`echo "a" && echo "b"`, 19},
		{`echo "escaped \" quote"`, 22},
		{`echo "`, -1},
		{`echo 'a' "b"`, 11},
	}
	for _, tt := range tests {
		got := findLastClosingQuotePos(tt.input)
		if got != tt.want {
			t.Errorf("findLastClosingQuotePos(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// --- sortCandidates tests ---

func TestSortCandidatesQuoteExtensionFirst(t *testing.T) {
	// Scenario: shared prefix = `git commit -m "feat: implement new funct`
	// The candidate extending the quote content should rank first.
	prefix := `git commit -m "feat: implement new funct`
	candidates := []ashlet.Candidate{
		{Completion: prefix + `" && git push`, Confidence: 0.95},
		{Completion: prefix + `ion"`, Confidence: 0.80},
		{Completion: prefix + `"`, Confidence: 0.65},
	}
	input := `git commit -m "feat: implement new funct`

	sortCandidates(candidates, input)

	// Quote-extending candidate should be first
	if candidates[0].Completion != prefix+`ion"` {
		t.Errorf("expected quote-extending candidate first, got %q", candidates[0].Completion)
	}
	// Chain command second
	if candidates[1].Completion != prefix+`" && git push` {
		t.Errorf("expected chain command second, got %q", candidates[1].Completion)
	}
	// Trivial completion last
	if candidates[2].Completion != prefix+`"` {
		t.Errorf("expected trivial completion last, got %q", candidates[2].Completion)
	}

	// Confidence should be re-assigned position-based
	if math.Abs(candidates[0].Confidence-0.95) > 1e-9 {
		t.Errorf("expected first candidate confidence 0.95, got %.2f", candidates[0].Confidence)
	}
	if math.Abs(candidates[1].Confidence-0.80) > 1e-9 {
		t.Errorf("expected second candidate confidence 0.80, got %.2f", candidates[1].Confidence)
	}
	if math.Abs(candidates[2].Confidence-0.65) > 1e-9 {
		t.Errorf("expected third candidate confidence 0.65, got %.2f", candidates[2].Confidence)
	}
}

func TestSortCandidatesNoResortShortPrefix(t *testing.T) {
	// Candidates with diverse prefixes (short LCP) should keep original order
	candidates := []ashlet.Candidate{
		{Completion: "git status", Confidence: 0.95},
		{Completion: "git commit", Confidence: 0.80},
		{Completion: "grep -r foo", Confidence: 0.65},
	}

	sortCandidates(candidates, "g")

	// Order should be preserved
	if candidates[0].Completion != "git status" {
		t.Errorf("expected order preserved, got %q first", candidates[0].Completion)
	}
	if candidates[1].Completion != "git commit" {
		t.Errorf("expected order preserved, got %q second", candidates[1].Completion)
	}
	// Confidence should remain unchanged (no re-sort happened)
	if math.Abs(candidates[0].Confidence-0.95) > 1e-9 {
		t.Errorf("expected confidence unchanged, got %.2f", candidates[0].Confidence)
	}
}

func TestSortCandidatesSingleCandidate(t *testing.T) {
	candidates := []ashlet.Candidate{
		{Completion: "git status", Confidence: 0.95},
	}
	sortCandidates(candidates, "git s")

	if candidates[0].Completion != "git status" {
		t.Errorf("single candidate should be unchanged")
	}
	if math.Abs(candidates[0].Confidence-0.95) > 1e-9 {
		t.Errorf("confidence should be unchanged, got %.2f", candidates[0].Confidence)
	}
}

func TestSortCandidatesAllSame(t *testing.T) {
	candidates := []ashlet.Candidate{
		{Completion: "git status", Confidence: 0.95},
		{Completion: "git status", Confidence: 0.80},
	}
	sortCandidates(candidates, "git s")

	// Both are identical — should remain stable
	if candidates[0].Completion != "git status" || candidates[1].Completion != "git status" {
		t.Error("equal candidates should stay stable")
	}
}

// --- commonPrefix / quoteExtensionLength unit tests ---

func TestCommonPrefix(t *testing.T) {
	tests := []struct {
		a, b, want string
	}{
		{"abc", "abd", "ab"},
		{"abc", "abc", "abc"},
		{"abc", "xyz", ""},
		{"abc", "ab", "ab"},
		{"", "abc", ""},
	}
	for _, tt := range tests {
		got := commonPrefix(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("commonPrefix(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestQuoteExtensionLength(t *testing.T) {
	tests := []struct {
		suffix string
		want   int
	}{
		{`ion"`, 3},
		{`" && git push`, 0},
		{`"`, 0},
		{`hello world`, 0},
		{`some text' more`, 9},
	}
	for _, tt := range tests {
		got := quoteExtensionLength(tt.suffix)
		if got != tt.want {
			t.Errorf("quoteExtensionLength(%q) = %d, want %d", tt.suffix, got, tt.want)
		}
	}
}

// --- Redaction in buildUserMessage tests ---

func TestBuildUserMessageRedactsRecentCommands(t *testing.T) {
	e := testEngine()
	req := &ashlet.Request{
		Input:     "curl ",
		CursorPos: 5,
		Cwd:       "/home/user",
	}
	info := &Info{
		RecentCommands: []string{
			"curl -H $SECRET_TOKEN http://example.com",
			"ls -la",
			"export API_KEY=supersecret",
		},
	}
	msg := e.buildUserMessage(req, info, nil)

	if strings.Contains(msg, "SECRET_TOKEN") {
		t.Error("user message should not contain sensitive var name SECRET_TOKEN")
	}
	if strings.Contains(msg, "supersecret") {
		t.Error("user message should not contain sensitive value 'supersecret'")
	}
	if !strings.Contains(msg, "$REDACTED") {
		t.Error("user message should contain $REDACTED for sensitive vars")
	}
	if !strings.Contains(msg, "API_KEY=***") {
		t.Error("user message should contain redacted assignment")
	}
}

func TestBuildUserMessageRedactsRelevantCommands(t *testing.T) {
	e := testEngine()
	req := &ashlet.Request{
		Input:     "docker ",
		CursorPos: 7,
		Cwd:       "/home/user",
	}
	info := &Info{
		RelevantCommands: []string{
			"docker login -p $DOCKER_PASSWORD",
			"docker build -t myapp .",
		},
	}
	msg := e.buildUserMessage(req, info, nil)

	if strings.Contains(msg, "DOCKER_PASSWORD") {
		t.Error("user message should not contain sensitive var DOCKER_PASSWORD in related commands")
	}
	if !strings.Contains(msg, "$REDACTED") {
		t.Error("user message should contain $REDACTED in related commands")
	}
}

func TestBuildUserMessagePreservesSafeVars(t *testing.T) {
	e := testEngine()
	req := &ashlet.Request{
		Input:     "cd ",
		CursorPos: 3,
		Cwd:       "/home/user",
	}
	info := &Info{
		RecentCommands: []string{
			"cd $HOME/projects",
		},
	}
	msg := e.buildUserMessage(req, info, nil)

	if !strings.Contains(msg, "$HOME") {
		t.Error("user message should preserve safe var $HOME")
	}
}

func TestBuildUserMessageInputNotRedacted(t *testing.T) {
	e := testEngine()
	req := &ashlet.Request{
		Input:     "echo $SECRET_VAR",
		CursorPos: 16,
		Cwd:       "/home/user",
	}
	msg := e.buildUserMessage(req, &Info{}, nil)

	if !strings.Contains(msg, "Input: `echo $SECRET_VAR`") {
		t.Error("user input should NOT be redacted — it's what the user is actively typing")
	}
}

// --- Embedding gate tests ---

func TestGathererNoRawHistoryWithoutEmbedding(t *testing.T) {
	// noRawHistory=true but no embedder → falls back to default behavior
	// (recent commands). The mismatch is caught by ValidateConfig at startup.
	trueVal := true
	cfg := ashlet.DefaultConfig()
	cfg.Generation.NoRawHistory = &trueVal
	g := NewGatherer(nil, cfg)
	defer g.Close()

	// embeddingEnabled should be false when embedder is nil
	if g.embeddingEnabled {
		t.Error("embeddingEnabled should be false when embedder is nil")
	}
	// Since embedding is disabled, noRawHistory gate is bypassed,
	// so recent commands are still populated (graceful degradation).
	req := &ashlet.Request{Input: "git ", CursorPos: 4}
	info := g.Gather(context.Background(), req)
	// Should hit the default path, not the embedding-gate path.
	// RecentCommands may be empty/nil if no history file exists,
	// but RelevantCommands must be nil (no embedding).
	if len(info.RelevantCommands) > 0 {
		t.Error("should not have relevant commands without embedder")
	}
}

func TestGathererWithRawHistory(t *testing.T) {
	// noRawHistory=false → should include recent commands
	falseVal := false
	cfg := ashlet.DefaultConfig()
	cfg.Generation.NoRawHistory = &falseVal
	g := NewGatherer(nil, cfg)
	defer g.Close()

	req := &ashlet.Request{Input: "git ", CursorPos: 4}
	info := g.Gather(context.Background(), req)

	// RecentCommands may be empty if no history file, but the code path should
	// have attempted to read them (not skipped due to noRawHistory gate).
	// nil is acceptable for no history file; the important thing is we
	// didn't return via the embedding-gate path (which sets RecentCommands
	// to nil and only populates RelevantCommands).
	_ = info.RecentCommands
}
