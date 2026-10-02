package client

import (
	"ai-chat-service/services"
	vpb "ai-chat-service/vecfluxapi/proto"
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn  *grpc.ClientConn
	api   vpb.VecFluxAPIClient
	token string
}

func New(address, accessToken string) (*Client, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, fmt.Errorf("vecfluxapi address is empty")
	}
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:  conn,
		api:   vpb.NewVecFluxAPIClient(conn),
		token: accessToken,
	}, nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) auth(ctx context.Context) context.Context {
	return services.AppendBearerTokenToContext(ctx, c.token)
}

func (c *Client) VSet(ctx context.Context, in *vpb.VSetRequest) (*vpb.VSetResponse, error) {
	return c.api.VSet(c.auth(ctx), in)
}

func (c *Client) VGet(ctx context.Context, in *vpb.VGetRequest) (vpb.VecFluxAPI_VGetClient, error) {
	return c.api.VGet(c.auth(ctx), in)
}
