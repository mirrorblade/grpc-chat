package service

import (
	"context"
	"errors"
	"grpctest/proto"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type ChatServer struct {
	sync.RWMutex
	proto.UnimplementedChatServer

	clients           map[string]*proto.ClientOptions
	clientsConnection map[string]proto.Chat_StreamServer

	logger *zap.Logger
}

func NewChatServer(logger *zap.Logger) *ChatServer {
	return &ChatServer{
		clients:           make(map[string]*proto.ClientOptions),
		clientsConnection: make(map[string]proto.Chat_StreamServer),
		logger:            logger,
	}
}

func (c *ChatServer) Run(ctx context.Context, host string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	grpcServer := grpc.NewServer()

	proto.RegisterChatServer(grpcServer, c)

	lis, err := net.Listen("tcp", host)
	if err != nil {
		c.logger.Error("invalid host")

		return err
	}

	c.logger.Info("server has started!")

	go func() {
		grpcServer.Serve(lis)
		cancel()
	}()

	<-ctx.Done()

	c.logger.Info("server is shutting down")

	grpcServer.GracefulStop()

	return nil
}

func (c *ChatServer) GetClients(_ context.Context, _ *emptypb.Empty) (*proto.GetClientsResponse, error) {
	c.RLock()
	defer c.RUnlock()

	return &proto.GetClientsResponse{
		Clients: c.clients,
	}, nil
}

func (c *ChatServer) Connect(_ context.Context, req *proto.ConnectRequest) (*proto.ConnectResponse, error) {
	clientUUID := uuid.New().String()

	c.RLock()
	if _, ok := c.clients[clientUUID]; ok {
		return nil, status.Error(codes.Internal, "server error")
	}
	c.RUnlock()

	username := req.GetUsername()
	if len(strings.TrimSpace(username)) == 0 {
		return nil, status.Error(codes.InvalidArgument, "empty username")
	}

	c.Lock()
	c.clients[clientUUID] = &proto.ClientOptions{
		Name: username,
	}
	c.Unlock()

	return &proto.ConnectResponse{
		Uuid: clientUUID,
	}, nil
}

func (c *ChatServer) Close(_ context.Context, req *proto.CloseRequest) (*proto.CloseResponse, error) {
	length := len(c.clients)

	c.Lock()
	delete(c.clients, req.GetUuid())
	c.Unlock()

	if length == len(c.clients) {
		return nil, status.Error(codes.InvalidArgument, "invalid uuid")
	}

	return &proto.CloseResponse{
		Text: "successful closing!",
	}, nil
}

func (c *ChatServer) Stream(stream proto.Chat_StreamServer) error {
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return errors.New("context error")
	}

	uuid := md.Get("uuid")[0]

	c.clientsConnection[uuid] = stream
	defer delete(c.clientsConnection, uuid)

	for {
		req, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}

		anotherStream, ok := c.clientsConnection[req.GetReceieverUuid()]
		if !ok {
			continue
		}

		if err := anotherStream.Send(&proto.StreamResponse{
			ReceieverUuid: req.GetReceieverUuid(),
			Text:          req.GetText(),
			Time:          req.GetTime(),
		}); err != nil {
			return err
		}
	}

	return nil
}
