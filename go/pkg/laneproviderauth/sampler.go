package laneproviderauth

// RFC 0162 Layer 1 sampler — a read-only file sample (no provider round-trip)
// that ties the F2 resolver contract to the expiry parser. It resolves the
// credential the lane CLI presents at runtime (via its launch env), reads it as
// the lane user, and extracts expiry. It FAILS CLOSED into ErrResolverMismatch
// when the runtime source cannot be proven, distinguishes a genuinely ABSENT
// credential (ErrCredentialAbsent → census absence, not a green gauge) from an
// unprovable one, and never reads a fresher HOME decoy as green (FA-F2 / FA-6).

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"
)

// ErrCredentialAbsent reports that the resolver-proven credential path does not
// exist — the credential genuinely vanished. The sampler surfaces this distinctly
// from ErrResolverMismatch: an absent credential emits no sample_present (so the
// per-lane census names the missing lane), whereas an unprovable source pages via
// resolver_mismatch.
var ErrCredentialAbsent = errors.New("lane credential is absent")

// LaneCredentialSample is one resolver-proven credential observation. Present is
// always true for a returned sample (the error returns carry the absence /
// mismatch cases). HasExpiry distinguishes an expiry-capable OAuth credential
// from a present-but-no-expiry api_key sample.
type LaneCredentialSample struct {
	Lane      string
	Provider  string
	Kind      string
	HasExpiry bool
	ExpiresAt time.Time
	IssuedAt  time.Time
}

// CredentialReader reads a credential file at path AS THE LANE USER, returning
// os.ErrNotExist when the file is absent. It is the injectable seam the daemon
// fold supplies (the sudo-as-lane reader) and tests supply in-memory.
type CredentialReader func(path string) ([]byte, error)

// SampleLaneCredential resolves, reads, and parses one lane's credential.
//
// It returns:
//   - (sample, nil)                  on a proven, readable, parseable credential;
//   - (zero, ErrCredentialAbsent)    when the resolved path does not exist;
//   - (zero, ErrResolverMismatch)    when the runtime source cannot be proven —
//     no resolver entry, no resolvable path, an unreadable file, or an
//     unparseable payload (fail-closed: a credential we cannot prove is not
//     emitted as green).
func SampleLaneCredential(entry RosterEntry, launchEnv []string, read CredentialReader) (LaneCredentialSample, error) {
	resolved, err := ResolveCredential(entry.Provider, entry.Kind, launchEnv)
	if err != nil {
		return LaneCredentialSample{}, ErrResolverMismatch
	}
	payload, err := read(resolved.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return LaneCredentialSample{}, ErrCredentialAbsent
		}
		// Unreadable for any other reason — cannot prove the source. Fail closed.
		return LaneCredentialSample{}, ErrResolverMismatch
	}
	expiry, err := ParseExpiry(entry.Provider, entry.Kind, payload)
	if err != nil {
		return LaneCredentialSample{}, ErrResolverMismatch
	}
	return LaneCredentialSample{
		Lane:      entry.Lane,
		Provider:  entry.Provider,
		Kind:      entry.Kind,
		HasExpiry: expiry.HasExpiry,
		ExpiresAt: expiry.ExpiresAt,
		IssuedAt:  expiry.IssuedAt,
	}, nil
}

// LaneFileReader builds the production CredentialReader: it reads the credential
// file AS THE LANE USER via the same `sudo -n -u <user> env -i … cat` shape the
// offline auth probe uses, so the daemon samples the lane-owned file even when it
// cannot read it directly. A no-such-file result maps to os.ErrNotExist; any
// other failure is returned as a generic error (→ resolver_mismatch). With an
// empty runAsUser it reads in-process. The read is best-effort and bounded by the
// caller's context.
func LaneFileReader(ctx context.Context, runner Runner, runAsUser string, launchEnv []string) CredentialReader {
	return func(path string) ([]byte, error) {
		runAsUser = strings.TrimSpace(runAsUser)
		if runAsUser == "" {
			return os.ReadFile(path) //nolint:gosec // resolver-proven lane credential path
		}
		spec := BuildLaunchSpec([]string{"cat", "--", path}, "", runAsUser, SanitizeEnv(launchEnv, nil))
		readCtx, cancel := context.WithTimeout(ctx, offlineAuthTimeout())
		defer cancel()
		res := runner.Run(readCtx, spec)
		if res.ExitCode == 0 && res.Err == nil {
			return []byte(res.Stdout), nil
		}
		text := strings.ToLower(res.Stderr + "\n" + errorString(res.Err))
		if strings.Contains(text, "no such file") || strings.Contains(text, "not found") {
			return nil, os.ErrNotExist
		}
		return nil, errors.New("lane credential read failed")
	}
}
