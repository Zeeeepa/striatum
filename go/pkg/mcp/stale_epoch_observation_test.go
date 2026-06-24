package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

// fakeIdentifier is a test BoundSessionIdentifier + Authorizer. It identifies ONLY
// the configured "good" bearer to a bound session; every other bearer is
// unattributable. As an Authorizer it DENIES everything (proving the rejection path
// runs pre-auth and the identity read grants nothing).
type fakeIdentifier struct {
	goodToken string
	repoID    string
	sessionID string
}

func (f fakeIdentifier) Authorize(required *rpc.Capability, repositoryID, token string) rpc.AuthContext {
	return rpc.AuthContext{RepositoryID: repositoryID, Decision: "denied", DenialReason: "token_invalid"}
}

func (f fakeIdentifier) IdentifyBoundSession(token string) (string, string, bool) {
	if token == f.goodToken && f.goodToken != "" {
		return f.repoID, f.sessionID, true
	}
	return "", "", false
}

// fakeStaleEpochRecorder records the calls so a test can assert what was (and was
// NOT) attributed.
type fakeStaleEpochRecorder struct {
	rejections [][3]string // {repoID, runID, sessionID}
	recoveries [][3]string
}

func (r *fakeStaleEpochRecorder) RecordStaleEpochRejection(_ context.Context, repositoryID, runID, sessionID string) error {
	r.rejections = append(r.rejections, [3]string{repositoryID, runID, sessionID})
	return nil
}

func (r *fakeStaleEpochRecorder) RecordStaleEpochRecovered(_ context.Context, repositoryID, runID, sessionID string) error {
	r.recoveries = append(r.recoveries, [3]string{repositoryID, runID, sessionID})
	return nil
}

func newStaleEpochTestHandler(t *testing.T, ident fakeIdentifier, rec *fakeStaleEpochRecorder) *HTTPHandler {
	t.Helper()
	return NewHTTPHandler(Service{
		Authorizer:         ident,
		BootEpoch:          "epoch-live",
		StaleEpochRecorder: rec,
	})
}

func staleEpochRequest(t *testing.T, bearer, presentedEpoch string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, EndpointPath,
		strings.NewReader(`{"jsonrpc":"2.0","id":"mut","method":"tools/call","params":{"name":"work.complete","repository_id":"repo_1","arguments":{"repository_id":"repo_1"}}}`))
	req.Host = "127.0.0.1:8765"
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if presentedEpoch != "" {
		req.Header.Set(HeaderBootEpoch, presentedEpoch)
	}
	return req
}

// TestStaleEpochRejectionRecordsUnrecoverableForPresentingSession is acceptance
// (a)/A2-attrib: a stale-epoch rejection with an ATTRIBUTABLE bearer records a
// daemon.stale_epoch_rotation observation for the bound session, AND the request is
// STILL rejected 403 (the identity read grants nothing).
func TestStaleEpochRejectionRecordsUnrecoverableForPresentingSession(t *testing.T) {
	rec := &fakeStaleEpochRecorder{}
	handler := newStaleEpochTestHandler(t, fakeIdentifier{goodToken: "lane.secret", repoID: "repo_1", sessionID: "sess_lane"}, rec)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, staleEpochRequest(t, "lane.secret", "epoch-stale-other-daemon"))

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (the request is STILL rejected)", w.Code)
	}
	if len(rec.rejections) != 1 {
		t.Fatalf("recorded rejections = %d, want exactly 1 attributed observation", len(rec.rejections))
	}
	if rec.rejections[0][0] != "repo_1" || rec.rejections[0][2] != "sess_lane" {
		t.Fatalf("observation attribution = %v, want {repo_1, _, sess_lane}", rec.rejections[0])
	}
}

// TestUnattributableStaleEpochRejectionRecordsNothing is A2-attrib (no over-fire):
// an UNATTRIBUTABLE bearer (one IdentifyBoundSession cannot bind to a session)
// records NO observation; the request is still rejected.
func TestUnattributableStaleEpochRejectionRecordsNothing(t *testing.T) {
	rec := &fakeStaleEpochRecorder{}
	handler := newStaleEpochTestHandler(t, fakeIdentifier{goodToken: "lane.secret", repoID: "repo_1", sessionID: "sess_lane"}, rec)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, staleEpochRequest(t, "stranger.secret", "epoch-stale-other-daemon"))

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	if len(rec.rejections) != 0 {
		t.Fatalf("recorded rejections = %d, want 0 (unattributable -> record nothing)", len(rec.rejections))
	}
}

// TestStaleEpochObservationNilRecorderIsNoop proves the feature is additive: a
// handler with NO recorder (every existing unit-test handler) still 403s and records
// nothing — the recycled-port behavior is unchanged.
func TestStaleEpochObservationNilRecorderIsNoop(t *testing.T) {
	handler := NewHTTPHandler(Service{
		Authorizer: fakeIdentifier{goodToken: "lane.secret", repoID: "repo_1", sessionID: "sess_lane"},
		BootEpoch:  "epoch-live",
		// StaleEpochRecorder intentionally nil.
	})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, staleEpochRequest(t, "lane.secret", "epoch-stale-other-daemon"))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 with a nil recorder", w.Code)
	}
}

// TestCurrentEpochReconnectSupersedesObservation is the Correction-1 supersede hook
// at the HTTP layer: a session that re-presents the CURRENT live epoch (it
// reconnected) drives RecordStaleEpochRecovered for its bound session (which clears
// any LIVE observation), and the request proceeds past the epoch gate.
func TestCurrentEpochReconnectSupersedesObservation(t *testing.T) {
	rec := &fakeStaleEpochRecorder{}
	handler := newStaleEpochTestHandler(t, fakeIdentifier{goodToken: "lane.secret", repoID: "repo_1", sessionID: "sess_lane"}, rec)
	w := httptest.NewRecorder()
	// Present the CURRENT (matching) epoch — the lane reconnected.
	handler.ServeHTTP(w, staleEpochRequest(t, "lane.secret", "epoch-live"))

	// The epoch gate passed (the denial below is from the fake Authorizer, NOT the
	// epoch check), and the supersede hook fired for the bound session.
	if len(rec.recoveries) != 1 {
		t.Fatalf("recorded recoveries = %d, want exactly 1 supersede on reconnect", len(rec.recoveries))
	}
	if rec.recoveries[0][2] != "sess_lane" {
		t.Fatalf("recovery attribution = %v, want session sess_lane", rec.recoveries[0])
	}
	// No stale-epoch rejection was recorded (the epoch matched).
	if len(rec.rejections) != 0 {
		t.Fatalf("recorded rejections = %d, want 0 on a matching-epoch request", len(rec.rejections))
	}
}

// TestNoEpochRequestSupersedesNothing proves a pre-#316 request that presents NO
// epoch neither records nor supersedes (it is not a reconnection signal).
func TestNoEpochRequestSupersedesNothing(t *testing.T) {
	rec := &fakeStaleEpochRecorder{}
	handler := newStaleEpochTestHandler(t, fakeIdentifier{goodToken: "lane.secret", repoID: "repo_1", sessionID: "sess_lane"}, rec)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, staleEpochRequest(t, "lane.secret", "")) // no epoch header
	if len(rec.rejections) != 0 || len(rec.recoveries) != 0 {
		t.Fatalf("a no-epoch request must neither record nor supersede; rejections=%d recoveries=%d", len(rec.rejections), len(rec.recoveries))
	}
}
