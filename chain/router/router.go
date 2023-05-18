package router

import (
	"context"

	"buf.build/gen/go/argus-labs/world-engine/grpc/go/router/v1/routerv1grpc"
	routerv1 "buf.build/gen/go/argus-labs/world-engine/protocolbuffers/go/router/v1"
	"google.golang.org/grpc"

	"github.com/argus-labs/world-engine/chain/router/errors"
)

type Result struct {
	Code    uint64
	Message []byte
}

type NamespaceClients map[string]routerv1grpc.MsgClient

//go:generate mockgen -source=router.go -package mocks -destination mocks/router.go
type Router interface {
	Send(ctx context.Context, namespace, sender string, msg []byte) (Result, error)
	RegisterNamespace(namespace, serverAddr string) error
}

var _ Router = &router{}

type router struct {
	namespaces NamespaceClients
}

func NewRouter(opts ...Option) Router {
	r := &router{}
	for _, opt := range opts {
		opt(r)
	}
	if r.namespaces == nil {
		r.namespaces = make(NamespaceClients)
	}
	return r
}

func (r *router) Send(ctx context.Context, namespace, sender string, msg []byte) (Result, error) {
	srv, ok := r.namespaces[namespace]
	if !ok {
		return Result{}, errors.ErrNamespaceNotFound(namespace)
	}
	msgSend := &routerv1.MsgSend{
		Sender:  sender,
		Message: msg,
	}
	res, err := srv.SendMsg(ctx, msgSend)
	if err != nil {
		return Result{
			Code:    errors.Failed,
			Message: []byte(err.Error()),
		}, err
	}
	// put bytes into proto message and send to server
	return Result{
		Code:    res.Code,
		Message: res.Message,
	}, nil
}

func (r *router) RegisterNamespace(namespace, serverAddr string) error {
	cc, err := grpc.Dial(serverAddr)
	if err != nil {
		return err
	}
	client := routerv1grpc.NewMsgClient(cc)
	r.namespaces[namespace] = client
	return nil
}