package turndriver

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoopGeneratesAndSaysOnOurTurn(t *testing.T) {
	conv := &fakeConversation{
		turns: []Turn{
			{Kind: TurnOurTurn, ConversationID: "conv_1", Topic: "topic", TurnIndex: 1, Transcript: []TranscriptEntry{{TurnIndex: 0, AuthorSessionID: "sess_a", Body: "hello"}}},
			{Kind: TurnClosed, ConversationID: "conv_1"},
		},
	}
	gen := &fakeGenerator{outputs: []string{" contribution "}}
	err := Loop{
		Conversation: conv,
		Generator:    gen,
		Options:      testOptions(),
	}.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(conv.says) != 1 || conv.says[0].body != "contribution" {
		t.Fatalf("says = %#v, want one sanitized contribution", conv.says)
	}
	if len(gen.contexts) != 1 {
		t.Fatalf("generator contexts = %d, want 1", len(gen.contexts))
	}
	got := gen.contexts[0]
	if got.Topic != "topic" || len(got.Transcript) != 1 || got.Transcript[0].AuthorSessionID != "sess_a" {
		t.Fatalf("generator context = %#v", got)
	}
}

func TestLoopWaitsWhenNotOurFloorAndExitsWhenConversationCloses(t *testing.T) {
	conv := &fakeConversation{
		turns: []Turn{
			{Kind: TurnNotOurFloor, ConversationID: "conv_1"},
			{Kind: TurnNoWork, ConversationID: "conv_1"},
		},
		showStates: []ConversationState{
			{ConversationID: "conv_1", State: "open"},
			{ConversationID: "conv_1", State: "closed"},
		},
	}
	sleeps := 0
	opts := testOptions()
	opts.Sleep = func(context.Context, time.Duration) error {
		sleeps++
		return nil
	}
	err := Loop{Conversation: conv, Generator: &fakeGenerator{}, Options: opts}.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if sleeps != 1 {
		t.Fatalf("sleeps = %d, want 1", sleeps)
	}
	if len(conv.says) != 0 {
		t.Fatalf("not our floor said unexpectedly: %#v", conv.says)
	}
}

func TestLoopSanitizesGeneratedContentBeforeSay(t *testing.T) {
	conv := &fakeConversation{turns: []Turn{{Kind: TurnOurTurn, ConversationID: "conv_1"}}}
	gen := &fakeGenerator{outputs: []string{" \x1b[31mhello\r\nworld\x00\t\x1b[0m "}}
	err := Loop{Conversation: conv, Generator: gen, Options: testOptions()}.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := conv.says[0].body; got != "hello\nworld" {
		t.Fatalf("said body = %q, want sanitized output", got)
	}
}

func TestLoopClosedEnvelopeExitsCleanly(t *testing.T) {
	conv := &fakeConversation{turns: []Turn{{Kind: TurnClosed, ConversationID: "conv_1"}}}
	gen := &fakeGenerator{}
	err := Loop{Conversation: conv, Generator: gen, Options: testOptions()}.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(gen.contexts) != 0 || len(conv.says) != 0 {
		t.Fatalf("closed loop generated=%d said=%d", len(gen.contexts), len(conv.says))
	}
}

func TestLoopGeneratorFailureAndEmptyOutputDoNotSay(t *testing.T) {
	for _, tc := range []struct {
		name    string
		outputs []string
		errs    []error
	}{
		{name: "error", errs: []error{errors.New("boom"), errors.New("boom again")}},
		{name: "empty", outputs: []string{"   ", "\x1b[31m\x1b[0m"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conv := &fakeConversation{turns: []Turn{{Kind: TurnOurTurn, ConversationID: "conv_1"}}}
			gen := &fakeGenerator{outputs: tc.outputs, errs: tc.errs}
			failures := 0
			opts := testOptions()
			opts.MaxGenerateAttempts = 2
			opts.OnFailure = func(context.Context, Failure) error {
				failures++
				return nil
			}
			err := Loop{Conversation: conv, Generator: gen, Options: opts}.Run(context.Background())
			if !errors.Is(err, ErrGenerationFailed) {
				t.Fatalf("Run() error = %v, want ErrGenerationFailed", err)
			}
			if len(conv.says) != 0 {
				t.Fatalf("generator failure said unexpectedly: %#v", conv.says)
			}
			if failures != 1 {
				t.Fatalf("failure callback count = %d, want 1", failures)
			}
		})
	}
}

func TestLoopTreatsNotYourTurnSayAsAlreadyAdvanced(t *testing.T) {
	conv := &fakeConversation{
		turns:  []Turn{{Kind: TurnOurTurn, ConversationID: "conv_1"}, {Kind: TurnClosed, ConversationID: "conv_1"}},
		sayErr: ErrNotYourTurn,
	}
	gen := &fakeGenerator{outputs: []string{"first"}}
	err := Loop{Conversation: conv, Generator: gen, Options: testOptions()}.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(conv.says) != 1 {
		t.Fatalf("says = %d, want one attempted say", len(conv.says))
	}
}

func TestConversationContextCarriesNoControlState(t *testing.T) {
	typ := reflect.TypeOf(ConversationContext{})
	if typ.NumField() != 2 {
		t.Fatalf("ConversationContext field count = %d, want 2", typ.NumField())
	}
	fields := map[string]bool{}
	for i := 0; i < typ.NumField(); i++ {
		fields[typ.Field(i).Name] = true
	}
	for _, name := range []string{"Topic", "Transcript"} {
		if !fields[name] {
			t.Fatalf("ConversationContext missing field %s", name)
		}
	}
	prompt := RenderPrompt(ConversationContext{
		Topic:      "topic",
		Transcript: []TranscriptEntry{{TurnIndex: 0, AuthorSessionID: "sess_a", Body: "body"}},
	})
	for _, forbidden := range []string{"lease_id", "job_id", "work_packet", "capability_token", "STRIATUM_MCP_TOKEN"} {
		if strings.Contains(strings.ToLower(prompt), forbidden) {
			t.Fatalf("prompt contains control-state marker %q:\n%s", forbidden, prompt)
		}
	}
}

func testOptions() Options {
	return Options{
		PollInterval:        time.Nanosecond,
		MaxGenerateAttempts: 1,
		MaxOutputBytes:      1024,
		Sleep: func(context.Context, time.Duration) error {
			return nil
		},
	}
}

type fakeConversation struct {
	turns      []Turn
	showStates []ConversationState
	sayErr     error
	says       []fakeSay
}

type fakeSay struct {
	conversationID string
	body           string
}

func (f *fakeConversation) AwaitTurn(context.Context) (Turn, error) {
	if len(f.turns) == 0 {
		return Turn{Kind: TurnClosed}, nil
	}
	turn := f.turns[0]
	f.turns = f.turns[1:]
	return turn, nil
}

func (f *fakeConversation) Say(_ context.Context, conversationID string, body string) error {
	f.says = append(f.says, fakeSay{conversationID: conversationID, body: body})
	return f.sayErr
}

func (f *fakeConversation) Show(context.Context, string) (ConversationState, error) {
	if len(f.showStates) == 0 {
		return ConversationState{State: "open"}, nil
	}
	state := f.showStates[0]
	f.showStates = f.showStates[1:]
	return state, nil
}

type fakeGenerator struct {
	outputs  []string
	errs     []error
	contexts []ConversationContext
}

func (f *fakeGenerator) Generate(_ context.Context, ctx ConversationContext) (string, error) {
	f.contexts = append(f.contexts, ctx)
	idx := len(f.contexts) - 1
	if idx < len(f.errs) && f.errs[idx] != nil {
		return "", f.errs[idx]
	}
	if idx < len(f.outputs) {
		return f.outputs[idx], nil
	}
	return "", nil
}
