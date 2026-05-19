package reads

import (
	"context"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// HandleDoctorBlobBlock exposes the Go-owned blob diagnostics as a focused
// read RPC so Python daemon doctor can merge the same block without
// reimplementing S3 probe logic.
func HandleDoctorBlobBlock(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, _ := envelope.Params["repository_id"].(string)
	return blobDoctorBlock(ctx, runner, repositoryID), nil
}
