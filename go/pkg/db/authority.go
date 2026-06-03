package db

import "context"

// AuthorityContext carries the per-transaction daemon-authority secret and the
// attribution labels that the RFC 0110 prelude installs into the mutation
// transaction. The secret is the only authority-bearing value; the rest are
// labels only (RFC 0110 §4.6, C-GUC-NONAUTH).
//
// In release N the authority registry does not exist yet, so Secret is empty:
// the prelude still installs the GUC (so its statement-order invariant is
// already enforced, T-SQL-ORDER) but nothing checks it, keeping release N
// behavior-neutral. Release N+1 wires a non-empty per-instance secret.
type AuthorityContext struct {
	// Secret is the RAM-only daemon-authority secret (empty until N+1).
	Secret string
	// RPCID is the originating RPC request id (attribution label).
	RPCID string
	// PrincipalID is the calling principal/client (attribution label).
	PrincipalID string
	// SessionID is the bound session, when any (attribution label).
	SessionID string
}

// authorityPreludeSQL is the single statement that opens every authorized
// mutation. The daemon-authority secret is set first (statement-order invariant
// T-SQL-ORDER), followed by the three attribution labels. All four are
// transaction-local (set_config(..., is_local => true)) so they vanish at the
// transaction boundary for every outcome — commit, rollback, cancel, timeout,
// or panic — which is what makes T-ATTR-RESET hold without a DISCARD on the hot
// path. It is issued exclusively through ExecBound (extended protocol), so the
// secret and labels never appear in pg_stat_activity query text
// (C-EXTENDED-AUTH-PRELUDE).
const authorityPreludeSQL = `SELECT ` +
	`set_config('striatum.daemon_auth', $1, true), ` +
	`set_config('striatum.rpc_id', $2, true), ` +
	`set_config('striatum.principal_id', $3, true), ` +
	`set_config('app.session_id', $4, true)`

// BeginAuthorizedMutation is the RFC 0110 authorized-mutation constructor
// (C-AUTH-TX-WRAPPER): it begins a transaction on runner and immediately issues
// the authority/attribution prelude as the transaction's first statement,
// returning the transaction with the prelude already installed. If the prelude
// fails the transaction is rolled back and the error is returned, so a mutation
// can never proceed without the prelude having run.
//
// It returns the original TxRunner (not a wrapper) so every concrete capability
// the handlers reach by type assertion — row Query, JSONB text encoding — is
// preserved. Authority being a constructor rather than a per-handler convention
// is what the G-MUTATION-TX guard enforces; the mutation-coupled audit append
// (release N+1) is layered at the withTx chokepoint, which holds the
// transaction and the attribution context together.
func BeginAuthorizedMutation(ctx context.Context, runner Runner, attr AuthorityContext) (TxRunner, error) {
	tx, err := runner.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	if err := issueAuthorityPrelude(ctx, tx, attr); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	return tx, nil
}

// issueAuthorityPrelude installs the authority secret and attribution labels on
// tx over the extended protocol. ExecBound is the sole prelude entry point
// (G-PRELUDE-MODE) and the sole writer of striatum.daemon_auth. A transaction
// that does not support extended-protocol exec is a canned test fake with no
// real session to label, so the prelude is skipped; the production transaction
// always implements boundExecer.
func issueAuthorityPrelude(ctx context.Context, tx TxRunner, attr AuthorityContext) error {
	be, ok := tx.(boundExecer)
	if !ok {
		return nil
	}
	return be.ExecBound(ctx, authorityPreludeSQL, attr.Secret, attr.RPCID, attr.PrincipalID, attr.SessionID)
}
