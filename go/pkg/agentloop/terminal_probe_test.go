package agentloop

import (
	"bytes"
	"strings"
	"testing"
)

func TestTerminalProbeReplies(t *testing.T) {
	const input = "\x1b[6n\x1b]10;?\x1b\\\x1b]11;?\x1b\\\x1b[?u\x1b[c"
	replies := strings.Join(terminalProbeReplies("", input), "")
	for _, want := range []string{
		"\x1b[1;1R",
		"\x1b]10;rgb:eeee/eeee/eeee\x1b\\",
		"\x1b]11;rgb:0000/0000/0000\x1b\\",
		"\x1b[?0u",
		"\x1b[?1;2c",
	} {
		if !strings.Contains(replies, want) {
			t.Fatalf("terminal probe replies missing %q in %q", want, replies)
		}
	}
}

func TestTerminalProbeRepliesAcrossChunks(t *testing.T) {
	replies := terminalProbeReplies("\x1b]10;?", "\x1b\\")
	if len(replies) != 1 || replies[0] != "\x1b]10;rgb:eeee/eeee/eeee\x1b\\" {
		t.Fatalf("split OSC foreground query replies = %#v", replies)
	}
}

func TestCopyPTYOutputWithTerminalRepliesKeepsOutputUntouched(t *testing.T) {
	input := "before\x1b[6nafter"
	var output bytes.Buffer
	var terminalInput bytes.Buffer
	if err := copyPTYOutputWithTerminalReplies(strings.NewReader(input), &output, &terminalInput, "codex"); err != nil {
		t.Fatalf("copy PTY output: %v", err)
	}
	if output.String() != input {
		t.Fatalf("output = %q, want %q", output.String(), input)
	}
	if terminalInput.String() != "\x1b[1;1R" {
		t.Fatalf("terminal input = %q", terminalInput.String())
	}
}

func TestPTYOutputRepliesToCodexWorkspaceTrustPromptOnce(t *testing.T) {
	input := "Do you trust the contents of this directory?\nPress enter to continue\nDo you trust the contents of this directory?"
	var output bytes.Buffer
	var terminalInput bytes.Buffer
	if err := copyPTYOutputWithTerminalReplies(strings.NewReader(input), &output, &terminalInput, "codex"); err != nil {
		t.Fatalf("copy PTY output: %v", err)
	}
	if output.String() != input {
		t.Fatalf("output = %q, want %q", output.String(), input)
	}
	if terminalInput.String() != "\r" {
		t.Fatalf("terminal input = %q, want CR", terminalInput.String())
	}
}

func TestPTYOutputRepliesToCodexWorkspaceTrustPromptAcrossChunks(t *testing.T) {
	replies := ptyOutputReplyState{adapter: "codex"}
	if got := replies.forChunk("Do you trust the contents"); len(got) != 0 {
		t.Fatalf("first chunk replies = %#v", got)
	}
	got := replies.forChunk(" of this directory?")
	if len(got) != 1 || got[0] != "\r" {
		t.Fatalf("second chunk replies = %#v", got)
	}
}

func TestPTYOutputRepliesToRenderedCodexWorkspaceTrustPrompt(t *testing.T) {
	renderedPrompt := "\x1b[3;3HDo\x1b[3;6Hyou\x1b[3;10Htrust\x1b[3;16Hthe\x1b[3;20Hcontents\x1b[3;29Hof\x1b[3;32Hthis\x1b[3;37Hdirectory?"
	replies := ptyOutputReplyState{adapter: "codex"}
	got := replies.forChunk(renderedPrompt)
	if len(got) != 1 || got[0] != "\r" {
		t.Fatalf("rendered trust prompt replies = %#v", got)
	}
}

func TestPTYOutputDoesNotReplyToWorkspaceTrustPromptForOtherAdapters(t *testing.T) {
	input := "Do you trust the contents of this directory?"
	var output bytes.Buffer
	var terminalInput bytes.Buffer
	if err := copyPTYOutputWithTerminalReplies(strings.NewReader(input), &output, &terminalInput, "agy"); err != nil {
		t.Fatalf("copy PTY output: %v", err)
	}
	if output.String() != input {
		t.Fatalf("output = %q, want %q", output.String(), input)
	}
	if terminalInput.String() != "" {
		t.Fatalf("terminal input = %q, want empty", terminalInput.String())
	}
}
