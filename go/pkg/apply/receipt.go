package apply

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Receipt struct {
	ApplyReceiptID       string  `json:"apply_receipt_id"`
	RepositoryID         string  `json:"repository_id"`
	RunID                string  `json:"run_id"`
	JobID                string  `json:"job_id"`
	PatchArtifactID      string  `json:"patch_artifact_id"`
	PatchDigestSHA256    string  `json:"patch_digest_sha256"`
	BaseTreeSHA256       string  `json:"base_tree_sha256"`
	AppliedTreeSHA256    *string `json:"applied_tree_sha256"`
	SigningKeyID         string  `json:"signing_key_id"`
	Signature            string  `json:"signature"`
	EvidenceArtifactPath string  `json:"evidence_artifact_path"`
	State                string  `json:"state"`
	DenialReason         *string `json:"denial_reason"`
}

type ReceiptQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func LoadReceipt(ctx context.Context, runner ReceiptQuerier, repositoryID string, receiptID string) (Receipt, error) {
	var receipt Receipt
	err := runner.QueryRow(
		ctx,
		`SELECT apply_receipt_id, repository_id, run_id, job_id,
		        patch_artifact_id, patch_digest_sha256, base_tree_sha256,
		        applied_tree_sha256, signing_key_id, signature,
		        evidence_artifact_path, state, denial_reason
		   FROM striatumd.apply_receipts
		  WHERE repository_id = $1 AND apply_receipt_id = $2`,
		repositoryID,
		receiptID,
	).Scan(
		&receipt.ApplyReceiptID,
		&receipt.RepositoryID,
		&receipt.RunID,
		&receipt.JobID,
		&receipt.PatchArtifactID,
		&receipt.PatchDigestSHA256,
		&receipt.BaseTreeSHA256,
		&receipt.AppliedTreeSHA256,
		&receipt.SigningKeyID,
		&receipt.Signature,
		&receipt.EvidenceArtifactPath,
		&receipt.State,
		&receipt.DenialReason,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Receipt{}, ErrReceiptMissing
	}
	return receipt, err
}
