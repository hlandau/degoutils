package rpcnexus

import "net/http"
import "github.com/gorilla/rpc/v2"
import "github.com/gorilla/rpc/v2/json2"

var Server *rpc.Server

const HTTPEndpoint = "/jr"

func init() {
	Server = rpc.NewServer()
	Server.RegisterCodec(json2.NewCodec(), "application/json-rpc")
	http.Handle(HTTPEndpoint, Server)
}
