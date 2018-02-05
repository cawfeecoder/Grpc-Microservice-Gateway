package kafka

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/Shopify/sarama"
)

type kafkaServices struct {
	ProducerService sarama.SyncProducer
	ConsumerService sarama.Consumer
	NodeID          string
}

type SyncService struct {
	ServiceName string `json: "service_name"`
	ConsulAddr  string `json: "consul_addr"`
	SenderID    string `json: "send_id"`
}

var KafkaServices *kafkaServices
var ServiceRegistryChan = make(chan SyncService)

func init() {
	producer := createProducer()
	consumer := createConsumer()

	KafkaServices = &kafkaServices{
		ProducerService: producer,
		ConsumerService: consumer,
	}
}

//Register NodeID for available in kafka filtering
func RegisterNodeID(nodeID string) {
	KafkaServices.NodeID = nodeID
}

func createProducer() sarama.SyncProducer {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5

	brokers := []string{"localhost:9092"}
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		panic(err)
	}
	return producer
}

func (services kafkaServices) SendMessage(topic string, key string, message string) {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.StringEncoder(message),
	}

	_, _, err := KafkaServices.ProducerService.SendMessage(msg)
	if err != nil {
		log.Printf("[ERROR]: %v", err)
	}
}

func createConsumer() sarama.Consumer {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true

	brokers := []string{"localhost:9092"}

	consumer, err := sarama.NewConsumer(brokers, config)
	if err != nil {
		panic(err)
	}
	return consumer
}

func (services kafkaServices) StartConsuming(topic string) {
	consume, err := KafkaServices.ConsumerService.ConsumePartition(topic, 0, sarama.OffsetNewest)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			select {
			case err := <-consume.Errors():
				fmt.Println(err)
			case msg := <-consume.Messages():
				if string(msg.Key) == "/sync" {
					var result SyncService
					json.Unmarshal(msg.Value, &result)
					if result.SenderID != KafkaServices.NodeID {
						go func() {
							ServiceRegistryChan <- result
						}()
					}
				}
			}
		}
	}()

}
