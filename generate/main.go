// Package generate orchestrates model inference to generate shell completions.
package generate

import (
	"context"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"

	ashlet "github.com/Paranoid-AF/ashlet"
	defaults "github.com/Paranoid-AF/ashlet/default"
	"github.com/Paranoid-AF/ashlet/index"
)

// DefaultMaxCandidates is used when the request does not specify a limit.
const DefaultMaxCandidates = 4

// Engine orchestrates context gathering and model inference for completions.
type Engine struct {
	gatherer     *Gatherer
	generator    *Generator
	dirCache     *DirCache
	config       *ashlet.Config
	customPrompt string // loaded custom prompt template (empty = use default)
}

// NewEngine creates a new completion engine.
func NewEngine() *Engine {
	cfg, err := ashlet.LoadConfig()
	if err != nil {
		slog.Warn("failed to load config, using defaults", "error", err)
		cfg = ashlet.DefaultConfig()
	}

	// Load custom prompt if available
	customPrompt := loadCustomPrompt()
	if customPrompt == "" {
		slog.Debug("no custom prompt, using built-in default")
	}

	// Create embedder if embedding is configured
	var embedder *index.Embedder
	if ashlet.EmbeddingEnabled(cfg) {
		embedder = index.NewEmbedder(
			ashlet.ResolveEmbeddingBaseURL(cfg),
			ashlet.ResolveEmbeddingAPIKey(cfg),
			ashlet.ResolveEmbeddingModel(cfg),
		)
	}

	// Create generator if API key is available
	var gen *Generator
	genAPIKey := ashlet.ResolveGenerationAPIKey(cfg)
	if genAPIKey != "" {
		gen = NewGenerator(
			ashlet.ResolveGenerationBaseURL(cfg),
			genAPIKey,
			ashlet.ResolveGenerationModel(cfg),
			cfg.Generation.APIType,
			cfg.Generation.MaxTokens,
			cfg.Generation.Temperature,
			cfg.Generation.Stop,
			ashlet.OpenRouterTelemetryEnabled(cfg),
		)
	} else {
		slog.Warn("generation API key not configured")
	}

	return &Engine{
		gatherer:     NewGatherer(embedder, cfg),
		generator:    gen,
		dirCache:     NewDirCache(),
		config:       cfg,
		customPrompt: customPrompt,
	}
}

// loadCustomPrompt loads a custom prompt template.
// Returns empty string if no custom prompt exists.
func loadCustomPrompt() string {
	promptPath := ashlet.PromptPath()
	data, err := os.ReadFile(promptPath)
	if err != nil {
		return ""
	}
	slog.Info("loaded custom prompt", "path", promptPath)
	return string(data)
}

// Close releases resources held by the engine.
func (e *Engine) Close() {
	if e.generator != nil {
		e.generator.Close()
	}
	if e.gatherer != nil {
		e.gatherer.Close()
	}
	if e.dirCache != nil {
		e.dirCache.Close()
	}
}

// WarmContext pre-populates the directory context cache for the given path.
func (e *Engine) WarmContext(ctx context.Context, cwd string) {
	e.dirCache.Gather(ctx, cwd)
}

// Complete processes a completion request and returns a response.
func (e *Engine) Complete(ctx context.Context, req *ashlet.Request) *ashlet.Response {
	// Check if API key is configured
	if e.generator == nil {
		return &ashlet.Response{
			Candidates: []ashlet.Candidate{},
			Error: &ashlet.Error{
				Code:    "not_configured",
				Message: "generation API key not configured; set ASHLET_GENERATION_API_KEY or run 'ashlet --config'",
			},
		}
	}

	// Strip trailing newlines the shell client appends as line terminators.
	req.Input = strings.TrimRight(req.Input, "\n")
	req.Cwd = strings.TrimRight(req.Cwd, "\n")

	// Clamp cursor position to the (now-trimmed) input length.
	if req.CursorPos > len(req.Input) {
		req.CursorPos = len(req.Input)
	}

	// Skip empty or whitespace-only input
	if strings.TrimSpace(req.Input) == "" {
		return &ashlet.Response{Candidates: []ashlet.Candidate{}}
	}

	info := e.gatherer.Gather(ctx, req)

	slog.Debug("context gathered",
		"recent_commands", strings.Join(info.RecentCommands, " | "),
		"relevant_commands", strings.Join(info.RelevantCommands, " | "),
	)

	// Check for cancellation before expensive inference
	if ctx.Err() != nil {
		return &ashlet.Response{Candidates: []ashlet.Candidate{}}
	}

	maxCandidates := req.MaxCandidates
	if maxCandidates <= 0 {
		maxCandidates = DefaultMaxCandidates
	}

	dirCtx := e.dirCache.Get(req.Cwd)

	systemPrompt := e.buildSystemPrompt(maxCandidates)
	userMessage := e.buildUserMessage(req, info, dirCtx)

	slog.Debug("prompt", "system", systemPrompt, "user", userMessage)

	output, err := e.generator.Generate(ctx, systemPrompt, userMessage)
	if err != nil {
		slog.Error("generation error", "error", err)
		return &ashlet.Response{
			Candidates: []ashlet.Candidate{},
			Error: &ashlet.Error{
				Code:    "api_error",
				Message: err.Error(),
			},
		}
	}

	input := strings.TrimLeft(req.Input, " \t")
	candidates := parseCandidates(output, input, maxCandidates)
	if candidates == nil {
		candidates = []ashlet.Candidate{}
	}

	// Always post-process quote filtering on candidates
	candidates = filterCandidateQuotes(candidates, input)
	sortCandidates(candidates, input)

	return &ashlet.Response{Candidates: candidates}
}

// PromptData holds the data passed to the prompt template.
type PromptData struct {
	MaxCandidates    int
	CWD              string
	RecentCommands   []string
	RelevantCommands []string
	InputBefore      string
	InputAfter       string
	Input            string
	DirListing       string
	DirManifests     map[string]string
	GitRoot          string
	GitRootListing   string
	GitStagedFiles   string
	GitLog           string
	GitManifests     map[string]string
	PackageManager   string
}

var promptFuncs = template.FuncMap{
	"bullet": func(items []string) string {
		if len(items) == 0 {
			return ""
		}
		var sb strings.Builder
		for _, item := range items {
			sb.WriteString("- ")
			sb.WriteString(item)
			sb.WriteString("\n")
		}
		return strings.TrimSuffix(sb.String(), "\n")
	},
	"join": func(items []string, sep string) string {
		return strings.Join(items, sep)
	},
}

// buildSystemPrompt renders the system prompt from the template.
func (e *Engine) buildSystemPrompt(maxCandidates int) string {
	tmplSrc := e.customPrompt
	if tmplSrc == "" {
		tmplSrc = defaults.DefaultPrompt
	}

	data := PromptData{
		MaxCandidates: maxCandidates,
	}

	t, err := template.New("prompt").Funcs(promptFuncs).Parse(tmplSrc)
	if err != nil {
		slog.Warn("failed to parse prompt template, falling back to default", "error", err)
		t, _ = template.New("prompt").Funcs(promptFuncs).Parse(defaults.DefaultPrompt)
	}

	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		slog.Warn("failed to execute prompt template, falling back to default", "error", err)
		t, _ = template.New("prompt").Funcs(promptFuncs).Parse(defaults.DefaultPrompt)
		buf.Reset()
		t.Execute(&buf, data)
	}

	return strings.TrimRight(buf.String(), " \t\n")
}

// buildUserMessage constructs the user message from context and input.
func (e *Engine) buildUserMessage(req *ashlet.Request, info *Info, dirCtx *DirContext) string {
	var sb strings.Builder

	if req.Cwd != "" {
		sb.WriteString("cwd: ")
		sb.WriteString(req.Cwd)
		sb.WriteString("\n")
	}

	if dirCtx != nil {
		if dirCtx.CwdListing != "" {
			sb.WriteString("files: ")
			sb.WriteString(dirCtx.CwdListing)
			sb.WriteString("\n")
		}
		if dirCtx.PackageManager != "" {
			sb.WriteString("pkg: ")
			sb.WriteString(dirCtx.PackageManager)
			sb.WriteString("\n")
		}
		if dirCtx.GitRoot != "" {
			sb.WriteString("git root: ")
			sb.WriteString(dirCtx.GitRoot)
			sb.WriteString("\n")
		}
		if dirCtx.GitRootListing != "" {
			sb.WriteString("project files: ")
			sb.WriteString(dirCtx.GitRootListing)
			sb.WriteString("\n")
		}
		if dirCtx.GitStagedFiles != "" {
			sb.WriteString("staged: ")
			sb.WriteString(dirCtx.GitStagedFiles)
			sb.WriteString("\n")
		}
		if dirCtx.GitLog != "" {
			sb.WriteString("log: ")
			sb.WriteString(dirCtx.GitLog)
			sb.WriteString("\n")
		}
		for name, content := range dirCtx.CwdManifests {
			sb.WriteString(name)
			sb.WriteString(": ")
			sb.WriteString(content)
			sb.WriteString("\n")
		}
		for name, content := range dirCtx.GitManifests {
			sb.WriteString(name)
			sb.WriteString(": ")
			sb.WriteString(content)
			sb.WriteString("\n")
		}
	}

	// Cap recent commands at 5
	limit := len(info.RecentCommands)
	if limit > 5 {
		limit = 5
	}
	recentCmds := filterQuoteContentSlice(index.RedactCommands(info.RecentCommands[:limit]))
	if len(recentCmds) > 0 {
		sb.WriteString("recent: ")
		sb.WriteString(strings.Join(recentCmds, ", "))
		sb.WriteString("\n")
	}

	relevantCmds := filterQuoteContentSlice(index.RedactCommands(info.RelevantCommands))
	if len(relevantCmds) > 0 {
		sb.WriteString("related: ")
		sb.WriteString(strings.Join(relevantCmds, ", "))
		sb.WriteString("\n")
	}

	before := req.Input[:req.CursorPos]
	after := req.Input[req.CursorPos:]

	sb.WriteString("\nInput: `")
	sb.WriteString(before)
	if len(after) > 0 {
		sb.WriteString("█")
	}
	sb.WriteString(after)
	sb.WriteString("`")

	return sb.String()
}

// candidateBlock represents a parsed <candidate> tag from model output.
type candidateBlock struct {
	typ     string // "replace" or "append"
	content string // inner content between tags
}

// commandTag represents a parsed <command> tag from model output.
type commandTag struct {
	text   string
	cursor int // byte offset for cursor, or -1 if not set
}

var (
	reCandidate = regexp.MustCompile(`(?s)<candidate[^>]*\btype="(replace|append)"[^>]*>(.*?)</candidate>`)
	reCommand   = regexp.MustCompile(`<command\s*>([^<]*)</command>`)
)

// parseCandidateBlocks extracts <candidate> blocks from model output.
func parseCandidateBlocks(output string) []candidateBlock {
	matches := reCandidate.FindAllStringSubmatch(output, -1)
	blocks := make([]candidateBlock, 0, len(matches))
	for _, m := range matches {
		blocks = append(blocks, candidateBlock{typ: m[1], content: m[2]})
	}
	return blocks
}

// parseCommands extracts <command> tags from a candidate block's inner content.
// Cursor position is determined by the █ sentinel character in the command text.
func parseCommands(content string) []commandTag {
	matches := reCommand.FindAllStringSubmatch(content, -1)
	cmds := make([]commandTag, 0, len(matches))
	for _, m := range matches {
		raw := m[1]
		cursor := -1
		if idx := strings.Index(raw, "█"); idx >= 0 {
			cursor = idx
			raw = raw[:idx] + raw[idx+len("█"):]
		}
		text := collapseSpaces(strings.TrimSpace(raw))
		if text != "" {
			cmds = append(cmds, commandTag{text: text, cursor: cursor})
		}
	}
	return cmds
}

// chainSeparator returns the string to insert between existing input and
// appended commands. If the input already ends with a chain operator
// (&&, ||, |, ;), only a space is added if needed. Otherwise " && ".
func chainSeparator(input string) string {
	trimmed := strings.TrimRight(input, " \t")
	for _, op := range []string{"&&", "||", "|", ";"} {
		if strings.HasSuffix(trimmed, op) {
			if strings.HasSuffix(input, " ") {
				return ""
			}
			return " "
		}
	}
	return " && "
}

func parseCandidates(output string, input string, max int) []ashlet.Candidate {
	blocks := parseCandidateBlocks(output)

	if len(blocks) == 0 {
		return parseCandidatesFallback(output, input, max)
	}

	var candidates []ashlet.Candidate
	seen := make(map[string]bool)

	for _, block := range blocks {
		if len(candidates) >= max {
			break
		}

		commands := parseCommands(block.content)
		if len(commands) == 0 {
			continue
		}

		// Join multiple commands with " && "
		parts := make([]string, len(commands))
		for i, cmd := range commands {
			parts[i] = cmd.text
		}
		joined := strings.Join(parts, " && ")

		var completion string
		var cursorOffset int
		switch block.typ {
		case "append":
			sep := chainSeparator(input)
			completion = input + sep + joined
			cursorOffset = len(input) + len(sep)
		default: // "replace"
			completion = joined
		}

		completion = strings.TrimSpace(completion)
		if completion == "" || seen[completion] {
			continue
		}
		seen[completion] = true

		// Use cursor from the first <command> that specifies one
		var cursorPos *int
		for _, cmd := range commands {
			if cmd.cursor >= 0 {
				pos := cmd.cursor + cursorOffset
				cursorPos = &pos
				break
			}
		}

		candidates = append(candidates, ashlet.Candidate{
			Completion: completion,
			Confidence: -1,
			CursorPos:  cursorPos,
		})
	}

	// Position-based confidence
	for i := range candidates {
		candidates[i].Confidence = 0.95 - float64(i)*0.15
		if candidates[i].Confidence < 0.1 {
			candidates[i].Confidence = 0.1
		}
	}

	return candidates
}

// parseCandidatesFallback handles model output without <autocomplete> tags.
// Accepts unmarked lines that share the first word with the input.
func parseCandidatesFallback(output string, input string, max int) []ashlet.Candidate {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	trimmedInput := strings.TrimSpace(input)
	var candidates []ashlet.Candidate
	seen := make(map[string]bool)

	for _, rawLine := range lines {
		if len(candidates) >= max {
			break
		}
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "$ ") || strings.HasPrefix(line, "<") {
			continue
		}

		candidate := strings.Trim(line, "`")
		candidate = strings.TrimSpace(candidate)

		var command string
		if trimmedInput == "" {
			command = candidate
		} else if firstWord(candidate) == firstWord(trimmedInput) {
			command = candidate
		} else {
			continue
		}

		command = collapseSpaces(strings.TrimSpace(command))

		if command == "" || seen[command] {
			continue
		}
		seen[command] = true

		candidates = append(candidates, ashlet.Candidate{
			Completion: command,
			Confidence: -1,
		})
	}

	// Position-based confidence
	for i := range candidates {
		candidates[i].Confidence = 0.95 - float64(i)*0.15
		if candidates[i].Confidence < 0.1 {
			candidates[i].Confidence = 0.1
		}
	}

	return candidates
}

// collapseSpaces replaces runs of multiple spaces with a single space.
func collapseSpaces(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if r == ' ' {
			if !prevSpace {
				buf.WriteByte(' ')
			}
			prevSpace = true
		} else {
			buf.WriteRune(r)
			prevSpace = false
		}
	}
	return buf.String()
}

// firstWord returns the first whitespace-delimited word of s.
func firstWord(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, ' '); i > 0 {
		return s[:i]
	}
	return s
}

// filterCandidateQuotes applies quote-content filtering to candidates based on input.
// If the input has no quotes: strip quote content from each candidate, deduplicate,
// and set CursorPos before the last closing quote (so cursor lands inside "").
// If the input has quotes: keep content as-is, set CursorPos before last closing quote.
func filterCandidateQuotes(candidates []ashlet.Candidate, input string) []ashlet.Candidate {
	if len(candidates) == 0 {
		return candidates
	}

	inputHasQuotes := strings.ContainsAny(input, "\"'")

	seen := make(map[string]bool, len(candidates))
	var out []ashlet.Candidate
	for _, c := range candidates {
		cmd := c.Completion
		if !inputHasQuotes {
			cmd = filterQuoteContent(cmd)
		}

		if seen[cmd] {
			continue
		}
		seen[cmd] = true

		cursorPos := c.CursorPos
		if cursorPos == nil {
			if pos := findLastClosingQuotePos(cmd); pos >= 0 {
				// Only position cursor inside quotes if nothing meaningful
				// follows the closing quote (e.g. "&&", "||", "| grep").
				// Otherwise leave cursor at end so user can edit the chain.
				afterQuote := strings.TrimSpace(cmd[pos+1:])
				if afterQuote == "" {
					cursorPos = &pos
				}
			}
		}

		out = append(out, ashlet.Candidate{
			Completion: cmd,
			Confidence: c.Confidence,
			CursorPos:  cursorPos,
		})
	}
	return out
}

// findLastClosingQuotePos scans for matched quote pairs and returns the byte
// index of the last closing quote, or -1 if none found.
func findLastClosingQuotePos(s string) int {
	lastClose := -1
	i := 0
	for i < len(s) {
		ch := s[i]
		if ch == '"' || ch == '\'' {
			quote := ch
			i++ // skip opening quote
			// Scan for matching closing quote
			for i < len(s) {
				if s[i] == '\\' && i+1 < len(s) {
					i += 2
					continue
				}
				if s[i] == quote {
					lastClose = i
					break
				}
				i++
			}
		}
		i++
	}
	return lastClose
}

// filterQuoteContent strips text inside quotes from a command string.
// Double-quoted content becomes "" and single-quoted content becomes ''.
// Handles escaped quotes inside quoted strings.
func filterQuoteContent(cmd string) string {
	var buf strings.Builder
	buf.Grow(len(cmd))
	i := 0
	for i < len(cmd) {
		ch := cmd[i]
		if ch == '"' || ch == '\'' {
			quote := ch
			buf.WriteByte(quote)
			i++
			// Skip content until matching unescaped closing quote
			for i < len(cmd) {
				if cmd[i] == '\\' && i+1 < len(cmd) {
					i += 2 // skip escaped character
					continue
				}
				if cmd[i] == quote {
					break
				}
				i++
			}
			// Write closing quote if found
			if i < len(cmd) {
				buf.WriteByte(quote)
				i++
			}
		} else {
			buf.WriteByte(ch)
			i++
		}
	}
	return buf.String()
}

// filterQuoteContentSlice applies filterQuoteContent to each element and deduplicates.
func filterQuoteContentSlice(cmds []string) []string {
	seen := make(map[string]bool, len(cmds))
	out := make([]string, 0, len(cmds))
	for _, cmd := range cmds {
		filtered := filterQuoteContent(cmd)
		if !seen[filtered] {
			seen[filtered] = true
			out = append(out, filtered)
		}
	}
	return out
}

// commonPrefix returns the longest common prefix of two strings.
func commonPrefix(a, b string) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:n]
}

// quoteExtensionLength returns the number of characters before the first
// closing quote (" or ') in suffix. Returns 0 if no quote found.
func quoteExtensionLength(suffix string) int {
	for i, ch := range suffix {
		if ch == '"' || ch == '\'' {
			return i
		}
	}
	return 0
}

// sortCandidates re-orders candidates using a weighted formula that favours
// candidates extending quote content. Candidates are only re-sorted when they
// share a sufficiently long common prefix; otherwise the original position-based
// ordering is preserved.
func sortCandidates(candidates []ashlet.Candidate, input string) {
	if len(candidates) < 2 {
		return
	}

	// Compute LCP of all candidates
	lcp := candidates[0].Completion
	for _, c := range candidates[1:] {
		lcp = commonPrefix(lcp, c.Completion)
		if lcp == "" {
			break
		}
	}

	// Threshold: candidates must share a meaningful prefix
	minLen := len(input) / 2
	if minLen < 3 {
		minLen = 3
	}
	if len(lcp) < minLen {
		return
	}

	// Compute raw scores
	type scored struct {
		idx int
		raw float64
	}
	scores := make([]scored, len(candidates))
	for i, c := range candidates {
		suffix := c.Completion[len(lcp):]
		suffixLen := float64(len(suffix))
		quoteExt := float64(quoteExtensionLength(suffix))
		scores[i] = scored{idx: i, raw: suffixLen*0.2 + quoteExt*0.8}
	}

	// Min-max normalization
	minRaw, maxRaw := scores[0].raw, scores[0].raw
	for _, s := range scores[1:] {
		if s.raw < minRaw {
			minRaw = s.raw
		}
		if s.raw > maxRaw {
			maxRaw = s.raw
		}
	}

	rangeRaw := maxRaw - minRaw
	type ranked struct {
		candidate ashlet.Candidate
		weight    float64
	}
	items := make([]ranked, len(candidates))
	for i, s := range scores {
		var normalized float64
		if rangeRaw > 0 {
			normalized = (s.raw - minRaw) / rangeRaw
		}
		weight := candidates[s.idx].Confidence*0.2 + 0.8*normalized
		items[i] = ranked{candidate: candidates[s.idx], weight: weight}
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].weight > items[j].weight
	})

	// Write back and re-assign position-based confidence
	for i, item := range items {
		candidates[i] = item.candidate
		candidates[i].Confidence = 0.95 - float64(i)*0.15
		if candidates[i].Confidence < 0.1 {
			candidates[i].Confidence = 0.1
		}
	}
}
