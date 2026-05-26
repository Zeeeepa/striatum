package turndriver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type TurnKind string

const (
	TurnOurTurn     TurnKind = "our_turn"
	TurnNotOurFloor TurnKind = "not_our_floor"
	TurnNoWork      TurnKind = "no_work"
	TurnClosed      TurnKind = "closed"
)

var (
	ErrEmptyOutput        = errors.New("turn-driver generator returned empty output")
	ErrGenerationFailed   = errors.New("turn-driver generator failed")
	ErrNotYourTurn        = errors.New("conversation floor already advanced")
	ErrUnexpectedEnvelope = errors.New("turn-driver received a workflow-control envelope")
)

type TranscriptEntry struct {
	TurnIndex       int
	AuthorSessionID string
	Body            string
}

type ConversationContext struct {
	Topic      string
	Transcript []TranscriptEntry
}

type Turn struct {
	Kind           TurnKind
	ConversationID string
	Topic          string
	Transcript     []TranscriptEntry
	TurnIndex      int
}

type ConversationState struct {
	ConversationID string
	State          string
}

func (s ConversationState) Closed() bool {
	return strings.EqualFold(strings.TrimSpace(s.State), "closed")
}

type Conversation interface {
	AwaitTurn(context.Context) (Turn, error)
	Say(context.Context, string, string) error
	Show(context.Context, string) (ConversationState, error)
}

type Generator interface {
	Generate(context.Context, ConversationContext) (string, error)
}

type Failure struct {
	Turn     Turn
	Err      error
	Attempts int
}

type Options struct {
	PollInterval          time.Duration
	GenerateTimeout       time.Duration
	MaxGenerateAttempts   int
	MaxOutputBytes        int
	InitialConversationID string
	Sleep                 func(context.Context, time.Duration) error
	OnFailure             func(context.Context, Failure) error
}

type Loop struct {
	Conversation Conversation
	Generator    Generator
	Options      Options
}

func (l Loop) Run(ctx context.Context) error {
	if l.Conversation == nil {
		return fmt.Errorf("turn-driver conversation seam is required")
	}
	if l.Generator == nil {
		return fmt.Errorf("turn-driver generator seam is required")
	}
	opts := l.options()
	knownConversationID := opts.InitialConversationID
	for {
		turn, err := l.Conversation.AwaitTurn(ctx)
		if err != nil {
			return err
		}
		if turn.ConversationID != "" {
			knownConversationID = turn.ConversationID
		}

		switch turn.Kind {
		case TurnOurTurn:
			if turn.ConversationID == "" {
				return fmt.Errorf("turn-driver our-turn envelope missing conversation_id")
			}
			body, attempts, err := l.generate(ctx, turn, opts)
			if err != nil {
				if opts.OnFailure != nil {
					_ = opts.OnFailure(ctx, Failure{Turn: turn, Err: err, Attempts: attempts})
				}
				return err
			}
			if err := l.Conversation.Say(ctx, turn.ConversationID, body); err != nil {
				if IsNotYourTurn(err) {
					continue
				}
				return err
			}
		case TurnNoWork, TurnNotOurFloor:
			conversationID := turn.ConversationID
			if conversationID == "" {
				conversationID = knownConversationID
			}
			if conversationID != "" {
				state, err := l.Conversation.Show(ctx, conversationID)
				if err != nil {
					return err
				}
				if state.Closed() {
					return nil
				}
			}
			if err := opts.Sleep(ctx, opts.PollInterval); err != nil {
				return err
			}
		case TurnClosed:
			return nil
		default:
			return fmt.Errorf("unknown turn kind %q", turn.Kind)
		}
	}
}

func (l Loop) generate(ctx context.Context, turn Turn, opts Options) (string, int, error) {
	var lastErr error
	for attempt := 1; attempt <= opts.MaxGenerateAttempts; attempt++ {
		genCtx := ctx
		cancel := func() {}
		if opts.GenerateTimeout > 0 {
			genCtx, cancel = context.WithTimeout(ctx, opts.GenerateTimeout)
		}
		output, err := l.Generator.Generate(genCtx, ConversationContext{
			Topic:      turn.Topic,
			Transcript: append([]TranscriptEntry(nil), turn.Transcript...),
		})
		cancel()
		if err == nil {
			body := SanitizeOutput(output, opts.MaxOutputBytes)
			if body != "" {
				return body, attempt, nil
			}
			err = ErrEmptyOutput
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = ErrGenerationFailed
	}
	return "", opts.MaxGenerateAttempts, fmt.Errorf("%w after %d attempt(s): %v", ErrGenerationFailed, opts.MaxGenerateAttempts, lastErr)
}

func (l Loop) options() Options {
	opts := l.Options
	if opts.PollInterval <= 0 {
		opts.PollInterval = time.Second
	}
	if opts.MaxGenerateAttempts <= 0 {
		opts.MaxGenerateAttempts = 2
	}
	if opts.MaxOutputBytes <= 0 {
		opts.MaxOutputBytes = 64 * 1024
	}
	if opts.Sleep == nil {
		opts.Sleep = func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		}
	}
	return opts
}

func IsNotYourTurn(err error) bool {
	if errors.Is(err, ErrNotYourTurn) {
		return true
	}
	type coded interface {
		ErrorCode() string
		Error() string
	}
	var c coded
	if errors.As(err, &c) {
		return c.ErrorCode() == "capability_denied" && strings.Contains(strings.ToLower(c.Error()), "not your turn")
	}
	return false
}

func SanitizeOutput(input string, maxBytes int) string {
	text := strings.ReplaceAll(input, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var builder strings.Builder
	used := 0
	for i := 0; i < len(text); {
		if text[i] == 0x1b {
			i = skipANSIEscape(text, i)
			continue
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == utf8.RuneError && size == 1 {
			i += size
			continue
		}
		i += size
		if isForbiddenControl(r) {
			continue
		}
		if maxBytes > 0 && used+size > maxBytes {
			break
		}
		builder.WriteRune(r)
		used += size
	}
	return strings.TrimSpace(builder.String())
}

func skipANSIEscape(s string, i int) int {
	if i+1 >= len(s) {
		return i + 1
	}
	switch s[i+1] {
	case '[':
		j := i + 2
		for j < len(s) {
			if s[j] >= 0x40 && s[j] <= 0x7e {
				return j + 1
			}
			j++
		}
		return len(s)
	case ']':
		j := i + 2
		for j < len(s) {
			if s[j] == 0x07 {
				return j + 1
			}
			if s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
				return j + 2
			}
			j++
		}
		return len(s)
	default:
		return i + 2
	}
}

func isForbiddenControl(r rune) bool {
	if r == '\n' || r == '\t' {
		return false
	}
	if r >= 0 && r < 0x20 {
		return true
	}
	return r == 0x7f || (r >= 0x80 && r <= 0x9f)
}

func RenderPrompt(ctx ConversationContext) string {
	var builder strings.Builder
	builder.WriteString("You are one participant in a Striatum multi-party conversation.\n")
	builder.WriteString("Write only your next contribution to the conversation. Do not describe tool use or workflow control.\n\n")
	builder.WriteString("Topic:\n")
	if strings.TrimSpace(ctx.Topic) == "" {
		builder.WriteString("(no topic provided)\n")
	} else {
		builder.WriteString(ctx.Topic)
		if !strings.HasSuffix(ctx.Topic, "\n") {
			builder.WriteByte('\n')
		}
	}
	builder.WriteString("\nTranscript so far:\n")
	if len(ctx.Transcript) == 0 {
		builder.WriteString("(no prior turns)\n")
	} else {
		for _, turn := range ctx.Transcript {
			builder.WriteString(fmt.Sprintf("Turn %d, %s:\n%s\n\n", turn.TurnIndex, turn.AuthorSessionID, turn.Body))
		}
	}
	builder.WriteString("Your next contribution:\n")
	return builder.String()
}
