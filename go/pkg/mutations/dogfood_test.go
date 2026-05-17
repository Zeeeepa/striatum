package mutations

import (
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestRegisterDogfoodAdapterInstallsCompositeHandlers(t *testing.T) {
	server := rpc.NewServer()
	RegisterDogfood(server, inertRunner{})

	for _, method := range []string{"dogfood.publish_on_behalf", "dogfood.surgical_recovery"} {
		if server.Handlers[method] == nil {
			t.Fatalf("%s was not registered", method)
		}
	}
}
