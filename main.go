package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	capi "github.com/hashicorp/consul/api"
	"github.com/kataras/iris"
	"github.com/kataras/iris/middleware/logger"
	"github.com/kataras/iris/middleware/recover"
	"github.com/liyue201/grpc-lb/registry/consul"
	"github.com/nfrush/grpc-graphql-restapi/kafka"
	"github.com/nfrush/grpc-graphql-restapi/proto"
	"github.com/nfrush/grpc-graphql-restapi/registry"
	proto "github.com/nfrush/grpc-microservice-gateway/proto"
	uuid "github.com/satori/go.uuid"
	"google.golang.org/grpc"
)

var nodeID uuid.UUID

var port int

type RegistryRPCServer struct {
	addr   string
	server *grpc.Server
}

func NewRegistryServer(addr string) *RegistryRPCServer {
	s := grpc.NewServer()
	rs := &RegistryRPCServer{
		addr:   addr,
		server: s,
	}
	return rs
}

func (rs *RegistryRPCServer) Run() {
	listener, err := net.Listen("tcp", rs.addr)
	if err != nil {
		log.Printf("[ERROR]: %v", err)
		return
	}
	log.Printf("[INFO]: Registry Service Listening On %s", rs.addr)

	GatewayProto.RegisterMicroserviceGatewayRegistryServer(rs.server, rs)
	rs.server.Serve(listener)
}

func (rs *RegistryRPCServer) Stop() {
	rs.server.GracefulStop()
}

func (rs *RegistryRPCServer) Send(ctx context.Context, req *GatewayProto.SvcReq) (*GatewayProto.SvcResp, error) {
	switch req.Action {
	case "Register":
		if err := registry.RegisterNewService(req.ServiceName, req.ServiceConsulAddr, nodeID.String()); err != nil {
			log.Printf("[ERROR]: %v", err)
			return &GatewayProto.SvcResp{StatusCode: 400, Message: err.Error()}, err
		}
		return &GatewayProto.SvcResp{StatusCode: 200}, nil
	}
	return &GatewayProto.SvcResp{StatusCode: 500, Message: "Internal Server Error"}, nil
}

func StartREST() {
	app := iris.New()
	app.Logger().SetLevel("debug")

	app.Use(recover.New())
	app.Use(logger.New())

	app.Handle("GET", "/*path", func(ctx iris.Context) {
		path := strings.Replace(ctx.Path(), "/", "", 1)
		client := registry.GetService(path)
		if client == nil {
			log.Println("Failed to find a service with name ", path)
			ctx.StatusCode(404)
			return
		} else {
			gctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
			resp, err := client.Client.Say(gctx, &proto.SayReq{Content: "consul"})
			if err != nil {
				log.Println(err)
				ctx.StatusCode(500)
				return
			}
			ctx.WriteString(resp.Content)
			return
		}
	})

	var randomPort = 11000 + rand.Intn(500)

	log.Printf("[INFO] REST Endpoint running at localhost:%d", randomPort)

	app.Run(iris.Addr(":"+strconv.Itoa(randomPort)), iris.WithoutServerError(iris.ErrServerClosed))
}

func StartService() {
	config := &capi.Config{
		Address: "http://localhost:8500",
	}

	registry, err := consul.NewRegistry(
		&consul.Congfig{
			ConsulCfg:   config,
			ServiceName: "microservice-gateway",
			NData: consul.NodeData{
				ID:      nodeID.String(),
				Address: "127.0.0.1",
				Port:    port,
			},
			Ttl: 5,
		})

	if err != nil {
		log.Panic(err)
		return
	}

	server := NewRegistryServer(fmt.Sprintf("0.0.0.0:%d", port))
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		StartREST()
		wg.Done()
	}()

	wg.Add(1)

	go func() {
		server.Run()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		registry.Register()
		wg.Done()
	}()

	kafka.KafkaServices.StartConsuming("microservice-gateway")
	wg.Wait()
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	nodeID = uuid.Must(uuid.NewV4())
	port = 12000 + rand.Intn(5000)

	kafka.RegisterNodeID(nodeID.String())

	StartService()
}
