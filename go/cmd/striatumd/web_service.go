package main

import (
	"net/http"

	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/webservice"
)

type webServiceOptions struct {
	RepositoryID    string
	CapabilityToken string
	ServiceToken    string
	AllowMutations  bool
	WebEnabled      bool
}

func newWebServiceHandler(rpcServer *rpc.Server, opts webServiceOptions) http.Handler {
	return webservice.New(webservice.Config{
		RPC:             rpcServer,
		RepositoryID:    opts.RepositoryID,
		CapabilityToken: opts.CapabilityToken,
		ServiceToken:    opts.ServiceToken,
		AllowMutations:  opts.AllowMutations,
		WebEnabled:      opts.WebEnabled,
	})
}
