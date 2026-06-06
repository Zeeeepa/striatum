package supervisor

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// gatedPacketReader serves the launch JSON object up front, then — on the next
// Read — waits a beat past the child's lifetime before delivering exactly one
// packet frame. This forces forwardPacketStream to emit its packet_accepted
// event right around the moment the child exits, which is when the child-exit
// branch of RunHelper (pre-fix) returned without joining the packet goroutine.
// After the frame it returns EOF so the goroutine can finish once joined.
//
// It deliberately does NOT call Wait() on the child process — RunHelper owns the
// sole Wait() — so the only cross-goroutine access this test introduces is the
// production race it is meant to catch (the packet_accepted emit against the
// caller reading the event buffer).
type gatedPacketReader struct {
	mu         sync.Mutex
	launch     []byte
	frame      []byte
	launchDone bool
	frameDone  bool
	gate       time.Duration
}

func (r *gatedPacketReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.launchDone {
		n := copy(p, r.launch)
		r.launch = r.launch[n:]
		if len(r.launch) == 0 {
			r.launchDone = true
		}
		return n, nil
	}
	if !r.frameDone {
		// Park past the child's lifetime so the frame's packet_accepted emit
		// coincides with RunHelper's child-exit return path.
		time.Sleep(r.gate)
		n := copy(p, r.frame)
		r.frame = r.frame[n:]
		if len(r.frame) == 0 {
			r.frameDone = true
		}
		return n, nil
	}
	return 0, io.EOF
}

// TestRunHelperJoinsPacketStreamOnChildExit is the #191 regression: the
// forwardPacketStream goroutine emits packet_accepted into the shared event
// buffer, and the child-exit branch of RunHelper returned without joining it, so
// the goroutine could emit concurrently with the caller reading the buffer — a
// data race under -race. RunHelper must join the packet goroutine before
// returning on child exit (mirroring the progress drain). Run under
// `go test -race` to catch a regression.
func TestRunHelperJoinsPacketStreamOnChildExit(t *testing.T) {
	origLaunch := helperLaunch
	defer func() { helperLaunch = origLaunch }()

	// A child that lives just long enough that forwardPacketStream is parked on
	// the gated reader when it exits.
	childCmd := exec.Command("sh", "-c", "sleep 0.05")
	if err := childCmd.Start(); err != nil {
		t.Fatalf("start child surrogate: %v", err)
	}
	helperLaunch = func(context.Context, string, string, LaunchSpec) (*LaunchResult, error) {
		return &LaunchResult{
			PID:         childCmd.Process.Pid,
			Cmd:         childCmd,
			StdinWriter: eofPTY{},
		}, nil
	}

	launch := HelperLaunchSpec{
		SchemaVersion: HelperLaunchSchemaVersion,
		SupervisorID:  "sup_packet_race",
		ScratchDir:    t.TempDir(),
		Command:       []string{"/bin/true"},
	}
	body, err := json.Marshal(launch)
	if err != nil {
		t.Fatal(err)
	}
	reader := &gatedPacketReader{
		launch: append(body, '\n'),
		frame:  []byte("packet-frame\n"),
		// Slightly longer than the child's 50ms life so the packet_accepted emit
		// lands around the child-exit return path.
		gate: 60 * time.Millisecond,
	}

	var events bytes.Buffer
	if err := RunHelper(context.Background(), reader, &events, HelperOptions{
		StdinEOFClosesPTY:  true,
		ProgressDrainDelay: 2 * time.Second,
	}); err != nil {
		t.Fatalf("RunHelper: %v\nevents=%s", err, events.String())
	}

	// Reading the buffer after RunHelper returns must be safe: the packet
	// goroutine has been joined, so its packet_accepted emit cannot race this
	// read. Under -race a pre-fix RunHelper trips the detector here.
	decoded, err := helperEventsFromJSONL(events.Bytes())
	if err != nil {
		t.Fatalf("decode events: %v\nraw=%s", err, events.String())
	}
	if len(decoded) == 0 {
		t.Fatalf("expected at least the agent_started event, got none")
	}
}
