package registry

import (
	"encoding/json"
	"log"

	grpclb "github.com/liyue201/grpc-lb"
	"github.com/liyue201/grpc-lb/registry/consul"
	"github.com/nfrush/grpc-graphql-restapi/kafka"
	"github.com/nfrush/grpc-microservice-gateway/proto"
	"google.golang.org/grpc"
)

type SyncService struct {
	ServiceName string `json: "service_name"`
	ConsulAddr  string `json: "consul_addr"`
	SenderID    string `json: "send_id"`
}

type RegistryService struct {
	Conn   *grpc.ClientConn    `json: conn`
	Client proto.ServiceClient `json: client`
}

var RegistryServices = make(map[string]*RegistryService)

func init() {
	go func() {
		var val kafka.SyncService
		val = <-kafka.ServiceRegistryChan
		RegisterNewService(val.ServiceName, val.ConsulAddr, val.SenderID)
	}()
}

func RegisterNewService(serviceName string, consulAddr string, nodeID string) error {
	_, ok := RegistryServices[serviceName]
	if ok {
		log.Printf("[INFO]: Consul Service has already been registered. This probably indicates a new service node has come online.")
		return nil
	}

	r := consul.NewResolver(serviceName, consulAddr)

	b := grpclb.NewBalancer(r, grpclb.NewRandomSelector())

	c, err := grpc.Dial("", grpc.WithInsecure(), grpc.WithBalancer(b))

	if err != nil {
		log.Printf("[ERROR]: %v", err)
		return err
	}

	client := proto.NewServiceClient(c)

	newService := &RegistryService{Conn: c, Client: client}

	RegistryServices[serviceName] = newService

	log.Printf("[INFO]: Succesfully registered new service %s", serviceName)

	if nodeID == kafka.KafkaServices.NodeID {
		log.Printf("[INFO]: Starting service sync")
		jsonString, err := json.Marshal(&SyncService{
			ServiceName: serviceName,
			ConsulAddr:  consulAddr,
			SenderID:    nodeID,
		})
		if err != nil {
			log.Printf("[ERROR]: %v", err)
		}
		kafka.KafkaServices.SendMessage("microservice-gateway", "/sync", string(jsonString))
		return nil
	}
	return nil
}

func GetService(serviceName string) *RegistryService {
	_, ok := RegistryServices[serviceName]

	if ok {
		return RegistryServices[serviceName]
	}
	return nil
}
