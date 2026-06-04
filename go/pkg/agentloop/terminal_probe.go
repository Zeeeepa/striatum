package agentloop

import (
	"io"
	"sort"
	"strings"
)

type terminalProbe struct {
	request  string
	response string
}

var terminalProbeResponses = []terminalProbe{
	{request: "\x1b[6n", response: "\x1b[1;1R"},                                // cursor position
	{request: "\x1b]10;?\x1b\\", response: "\x1b]10;rgb:eeee/eeee/eeee\x1b\\"}, // foreground color
	{request: "\x1b]10;?\a", response: "\x1b]10;rgb:eeee/eeee/eeee\x1b\\"},
	{request: "\x1b]11;?\x1b\\", response: "\x1b]11;rgb:0000/0000/0000\x1b\\"}, // background color
	{request: "\x1b]11;?\a", response: "\x1b]11;rgb:0000/0000/0000\x1b\\"},
	{request: "\x1b[?u", response: "\x1b[?0u"},  // keyboard protocol mode
	{request: "\x1b[c", response: "\x1b[?1;2c"}, // primary device attributes
}

type ptyOutputReplyState struct {
	adapter                       string
	tail                          string
	codexWorkspaceTrustPromptDone bool
}

func copyPTYOutputWithTerminalReplies(input io.Reader, output io.Writer, terminalInput io.Writer, adapter string) error {
	buf := make([]byte, 4096)
	replies := ptyOutputReplyState{adapter: normalizePTYReplyAdapter(adapter)}
	for {
		n, err := input.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			if _, writeErr := output.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			for _, response := range replies.forChunk(chunk) {
				if _, writeErr := io.WriteString(terminalInput, response); writeErr != nil {
					return writeErr
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func normalizePTYReplyAdapter(adapter string) string {
	adapter = strings.TrimSpace(adapter)
	if adapter == "" {
		return ""
	}
	return LaneAdapterName(adapter)
}

func (r *ptyOutputReplyState) forChunk(chunk string) []string {
	window := r.tail + chunk
	replies := terminalProbeReplies(r.tail, chunk)
	if r.adapter == "codex" && !r.codexWorkspaceTrustPromptDone && containsCodexWorkspaceTrustPrompt(window) {
		replies = append(replies, "\r")
		r.codexWorkspaceTrustPromptDone = true
	}
	r.tail = terminalProbeTail(window)
	return replies
}

func containsCodexWorkspaceTrustPrompt(s string) bool {
	compact := compactTerminalText(s)
	return strings.Contains(compact, "Doyoutrustthecontentsofthisdirectory?") ||
		strings.Contains(compact, "Pressentertocontinue")
}

func compactTerminalText(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		switch b := s[i]; {
		case b == '\x1b':
			i = skipEscapeSequence(s, i)
		case b > ' ' && b < 0x7f:
			out.WriteByte(b)
			i++
		default:
			i++
		}
	}
	return out.String()
}

func skipEscapeSequence(s string, i int) int {
	i++
	if i >= len(s) {
		return i
	}
	switch s[i] {
	case '[':
		i++
		for i < len(s) {
			b := s[i]
			i++
			if b >= 0x40 && b <= 0x7e {
				return i
			}
		}
		return i
	case ']':
		i++
		for i < len(s) {
			if s[i] == '\a' {
				return i + 1
			}
			if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2
			}
			i++
		}
		return i
	default:
		return i + 1
	}
}

func terminalProbeReplies(tail, chunk string) []string {
	window := tail + chunk
	oldLen := len(tail)
	type match struct {
		pos      int
		response string
	}
	var matches []match
	for _, probe := range terminalProbeResponses {
		start := 0
		for {
			pos := indexFrom(window, probe.request, start)
			if pos < 0 {
				break
			}
			if pos+len(probe.request) > oldLen {
				matches = append(matches, match{pos: pos, response: probe.response})
			}
			start = pos + len(probe.request)
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].pos < matches[j].pos })
	replies := make([]string, 0, len(matches))
	for _, match := range matches {
		replies = append(replies, match.response)
	}
	return replies
}

func terminalProbeTail(s string) string {
	const maxTail = 8192
	if len(s) <= maxTail {
		return s
	}
	return s[len(s)-maxTail:]
}

func indexFrom(s, substr string, start int) int {
	if start >= len(s) {
		return -1
	}
	for i := start; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
