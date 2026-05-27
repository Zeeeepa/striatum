package agentloop

import (
	"os"
	"testing"
)

func TestAgentLoopSubmitSequenceDefault(t *testing.T) {
	os.Unsetenv("STRIATUM_AGENT_LOOP_SUBMIT_SEQUENCE")
	if got := agentLoopSubmitSequence(); got != "\r" {
		t.Fatalf("default submit sequence = %q, want carriage return", got)
	}
}

func TestAgentLoopSubmitSequenceOverride(t *testing.T) {
	cases := map[string]string{
		`\n`:   "\n",
		`\r\n`: "\r\n",
		"":     "", // explicit empty disables the submit (headless EOF adapters)
		`\r`:   "\r",
		`x\ty`: "x\ty",
	}
	for raw, want := range cases {
		t.Setenv("STRIATUM_AGENT_LOOP_SUBMIT_SEQUENCE", raw)
		if got := agentLoopSubmitSequence(); got != want {
			t.Fatalf("submit sequence for %q = %q, want %q", raw, got, want)
		}
	}
}
