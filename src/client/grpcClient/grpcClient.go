package grpcclient

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"gophKeeper/src/client/cfg"
	pb "gophKeeper/src/pb"
)

var ErrAlreadyLoggedIn = errors.New("user already logged in")

func New(config *cfg.ConfigT) (client *GRPCClient, callback func() error) {
	client = &GRPCClient{
		config: config,
	}
	conn, err := grpc.Dial(
		config.ServerAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(client.Authenticate),
	)
	if err != nil {
		panic("connection refused")
	}

	client.auth = pb.NewAuthClient(conn)
	client.keep = pb.NewGophKeeperClient(conn)
	return client, conn.Close
}

type GRPCClient struct {
	config    *cfg.ConfigT
	auth      pb.AuthClient
	keep      pb.GophKeeperClient
	sessionID string
}

func (c *GRPCClient) Authenticate(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) (err error) {
	if strings.Contains(strings.ToLower(method), "base.auth") {
		err = invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			return fmt.Errorf("authenticate middleware bypass: %w", err)
		}
		return nil
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "sid", c.sessionID)
	err = invoker(ctx, method, req, reply, cc, opts...)
	if err != nil {
		return fmt.Errorf("authenticate middleware error: %w", err)
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errors.New("no metadata")
	}
	c.sessionID = md.Get("sid")[0]
	return nil
}

func (c *GRPCClient) Login(ctx context.Context, login, password string) error {
	resp, err := c.auth.Login(ctx, &pb.Credentials{
		Login:    login,
		Password: password,
	})
	if err != nil {
		if e, ok := status.FromError(err); ok {
			if e.Code() == codes.AlreadyExists {
				return ErrAlreadyLoggedIn
			}
		}
		return fmt.Errorf("grpc call login: %w", err)
	}
	c.sessionID = resp.GetSID()
	return nil
}

func (c *GRPCClient) KickOtherSession(ctx context.Context, login, password string) error {
	resp, err := c.auth.KickOtherSession(ctx, &pb.Credentials{
		Login:    login,
		Password: password,
	})
	if err != nil {
		return fmt.Errorf("grpc call kick other session: %w", err)
	}
	c.sessionID = resp.GetSID()
	return nil
}

func (c *GRPCClient) Register(ctx context.Context, login, password string) error {
	_, err := c.auth.Register(ctx, &pb.Credentials{
		Login:    login,
		Password: password,
	})
	if err != nil {
		return fmt.Errorf("register returned error: %w", err)
	}
	return nil
}

func (c *GRPCClient) Ping(ctx context.Context) error {
	_, err := c.keep.Ping(ctx, &pb.Empty{})
	if err != nil {
		return fmt.Errorf("gRPC ping returned error: %w", err)
	}
	return nil
}

type CategoryEntry struct {
	DataID   string
	Metadata string
}

type Category pb.Category

const (
	CategoryCred Category = Category(pb.Category_CATEGORY_CRED)
	CategoryText Category = Category(pb.Category_CATEGORY_TEXT)
	CategoryCard Category = Category(pb.Category_CATEGORY_CARD)
	CategoryFile Category = Category(pb.Category_CATEGORY_FILE)
)

func (c *GRPCClient) GetCategoryHead(ctx context.Context, category Category) (head []*CategoryEntry, err error) {
	resp, err := c.keep.GetCategoryHead(ctx, &pb.CategoryType_DTO{
		Category: pb.Category(category),
	})
	if err != nil {
		return nil, fmt.Errorf("grpc call get category head: %w", err)
	}
	info := resp.GetInfo()
	head = make([]*CategoryEntry, 0, len(info))
	for _, el := range info {
		head = append(head, &CategoryEntry{
			DataID:   el.GetDataID(),
			Metadata: el.GetMetadata(),
		})
	}
	return head, nil
}

func (c *GRPCClient) StoreCredentials(ctx context.Context, data []byte, meta string,
) (dataID, metadata string, err error) {
	resp, err := c.keep.StoreCredentials(ctx, &pb.SecureData_DTO{
		Data:     data,
		Metadata: meta,
	})
	if err != nil {
		return "", "", fmt.Errorf("grpc call store creds: %w", err)
	}
	return resp.GetID(), meta, nil
}

func (c *GRPCClient) LoadCredentials(ctx context.Context, dataID string) (data []byte, err error) {
	resp, err := c.keep.LoadCredentials(ctx, &pb.DataID_DTO{
		ID: dataID,
	})
	if err != nil {
		return nil, fmt.Errorf("grpc call load creds: %w", err)
	}
	return resp.GetData(), err
}

func (c *GRPCClient) StoreTextData(ctx context.Context, data []byte, meta string) (dataID, metadata string, err error) {
	resp, err := c.keep.StoreTextData(ctx, &pb.SecureData_DTO{
		Data:     data,
		Metadata: meta,
	})
	if err != nil {
		return "", "", fmt.Errorf("grpc call load text: %w", err)
	}
	return resp.GetID(), meta, err
}

func (c *GRPCClient) LoadTextData(ctx context.Context, dataID string) (data []byte, err error) {
	resp, err := c.keep.LoadTextData(ctx, &pb.DataID_DTO{
		ID: dataID,
	})
	if err != nil {
		return nil, fmt.Errorf("grpc call load text: %w", err)
	}
	return resp.GetData(), err
}

func (c *GRPCClient) StoreCreditCard(ctx context.Context, data []byte, meta string,
) (dataID, metadata string, err error) {
	resp, err := c.keep.StoreCreditCard(ctx, &pb.SecureData_DTO{
		Data:     data,
		Metadata: meta,
	})
	if err != nil {
		return "", "", fmt.Errorf("grpc call load card: %w", err)
	}
	return resp.GetID(), meta, err
}

func (c *GRPCClient) LoadCreditCard(ctx context.Context, dataID string) (data []byte, err error) {
	resp, err := c.keep.LoadCreditCard(ctx, &pb.DataID_DTO{
		ID: dataID,
	})
	if err != nil {
		return nil, fmt.Errorf("grpc call load card: %w", err)
	}
	return resp.GetData(), err
}
