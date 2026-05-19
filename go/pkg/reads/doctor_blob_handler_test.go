package reads

import (
	"context"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestHandleDoctorBlobBlockReturnsNotConfiguredWhenClientMissing(t *testing.T) {
	previous := packageBlobClient
	packageBlobClient = nil
	t.Cleanup(func() { packageBlobClient = previous })

	got, err := HandleDoctorBlobBlock(context.Background(), &doctorFakeRunner{}, rpc.Envelope{
		Params: map[string]any{},
	})
	if err != nil {
		t.Fatalf("HandleDoctorBlobBlock returned error: %v", err)
	}
	if got["configured"] != false {
		t.Fatalf("expected configured=false, got %#v", got)
	}
}

func TestRegisterExposesDoctorBlobBlockMethod(t *testing.T) {
	server := rpc.NewServer()
	Register(server, &doctorFakeRunner{})

	if _, ok := server.Handlers["doctor.blob_block"]; !ok {
		t.Fatalf("doctor.blob_block handler was not registered")
	}
	if _, ok := rpc.MethodRegistry["doctor.blob_block"]; !ok {
		t.Fatalf("doctor.blob_block method metadata was not registered")
	}
}
