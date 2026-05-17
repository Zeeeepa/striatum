package mutations

import (
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/dogfood"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

func RegisterDogfood(server *rpc.Server, runner db.Runner) {
	dogfood.Service{Runner: runner}.Register(server)
}
