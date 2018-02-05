# Golang Microservice Gateway

## Technologies Utilized: Iris(Golang MVC Framework), Kafka(Distributed Messaging Framework), HAProxy(Load Balancing), GraphQL(Provides a Graph Data Querying in place of traditional REST)

## Future Technology Considerations: Prometheus(For Request Auditing/Monitoring), Graphana(Visualize Prometheus Events), MongoDB(Storage for various Microservices), Redis(Storage for JWTs), SOME JavaScript Framework(Applicable Frontend for Testing), Apache Ignite/Nginx/Varnish(For Delivering JavaScript Frontend with minimized latency), Rancher/Kubernetes/Mesos(A container orchtestration service to provide autoscaling and autorestart for service uptime)

## Diagram

![alt-text][diagram]

[diagram]: https://i.imgur.com/tkXxTqo.png "Architecture Daigram"

## Architecture Overview
This project seeks to provide a seamless way to provide LB balancing among various microservices working together in a larger application. Moreover, the project seeks to rectify the idea of "living" services - microservices which may stop, go down, startup, fail, or any other number of actions - and easily proxy requests to these services, or handle the failures gracefully.

We start off using HAProxy to provide a singular exposed endpoint for the possiblity of n Microservice Gateways. Each of these gateways exposes either a REST or GraphQL endpoint(s), depending on the larger application requirements. THe ability to load balance between n of these, each knowing of ALL microservices available, allows us to minimize the response time back to a client. Notice, HAProxy can be configured to provide cookie based routing in the cases of WebSockets or other requests which are required to go to the same originating server.

Each Microserivce Gateway is defined by a number of applicable, unexposed services: Kafka, gRPC, and Consul. Microservices Gateways will utilize Kafka topics in order to synchronize the registering services among each other. In fact, these register services are Consul services themselves, allowing a blindness to the number of instances that may be backing a single service. During a service registration, a microservice will contact the Consul Service for the Microservice Gateways(Green) and ask to register a service. This request will be load balanced to 1 of n Microservice Gateways. After registering the service on that instance, a Kafka message will be broadcast - filtering by a unique NodeID to prevent a registration of the service by the originating gateway - causing all other microservices to "register" that service with themselves and bring the entire cluster in sync. If another instance of the same services decides to register, we'll transparently accept the request since we already have the Consul service registered and need not re-register. In cases where requests coming through the Gateway recieve a connection error to the Consul Service load balancing a service - an indication that all microservice instances are down - we'll seamless remove the service and resync. In the future, other services, such as Request/Response auditing, may happen in a similar fashion without an exposition of the auditing service to an endpoint. 

Notice, microservices are language agnostic(any language that supports gRPC is fair game... Rust, Kotlin, Elixir, Haskell, Scala, etc.). The Golang powered Gateways only concern themselves with an applicable GRPC connection and unifying protobuf definition(Each service will have to use a "generic" protobuf definition and implement it's specific logic on handling these types).

## Protobuf Definition(Unifying) - DRAFT
```proto
//protoc --go_out=plugins=grpc:. service.proto

package proto;

message ForwardingReq {
  int32 statusCode = 1;
  ... All applicable HTTP Request fields should be enumerated here
}

message ForwardingResp {
  int32 statusCode = 1;
  ... All applicable HTTP Response fields should be enumerated here
}

service ForwardingService {
  rpc Send(ForwardingReq) returns (ForwardingResp) {}
}
```
