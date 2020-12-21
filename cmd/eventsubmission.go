package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/filetrust/event-submission-service/pkg"
	"net/url"
	"github.com/streadway/amqp"
)

var (
	exchange   = "adaptation-exchange"
	routingKey = "transaction-event"
	queueName  = "transaction-event-queue"

	rootPath                      = os.Getenv("TRANSACTION_STORE_ROOT_PATH")
	transactionEventQueueHostname = os.Getenv("TRANSACTION_EVENT_QUEUE_HOSTNAME")
	transactionEventQueuePort     = os.Getenv("TRANSACTION_EVENT_QUEUE_PORT")
	messagebrokeruser             = os.Getenv("MESSAGE_BROKER_USER")
	messagebrokerpassword         = os.Getenv("MESSAGE_BROKER_PASSWORD")
)

const AnalysisReportID = 112

func main() {
	if rootPath == "" {
		log.Fatalf("init failed: TRANSACTION_STORE_ROOT_PATH")
	}

	if transactionEventQueueHostname == "" || transactionEventQueuePort == "" {
		log.Fatalf("init failed: TRANSACTION_EVENT_QUEUE_HOSTNAME, or TRANSACTION_EVENT_QUEUE_PORT environment variables not set")
	}

	if messagebrokeruser == "" {
		messagebrokeruser = "guest"
		log.Printf("Using default message broker user")
	}

	if messagebrokerpassword == "" {
		messagebrokerpassword = "guest"
		log.Printf("Using default message broker password")
	}

	amqpUrl := url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(messagebrokeruser, messagebrokerpassword),
		Host:   fmt.Sprintf("%s:%s", transactionEventQueueHostname, transactionEventQueuePort),
		Path:   "/",
	}

	fmt.Println("Connecting to ", amqpUrl.Host)

	conn, err := amqp.Dial(amqpUrl.String())
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	err = ch.ExchangeDeclare(exchange, "direct", true, false, false, false, nil)
	failOnError(err, "Failed to declare an exchange")

	q, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
	failOnError(err, "Failed to declare a queue")

	err = ch.QueueBind(q.Name, routingKey, exchange, false, nil)
	failOnError(err, "Failed to bind queue")

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	failOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			requeue, err := processMessage(d)
			if err != nil {
				log.Printf("Failed to process message: %v", err)
				ch.Nack(d.DeliveryTag, false, requeue)
			}
		}
	}()

	log.Printf("[*] Waiting for messages. To exit press CTRL+C")
	<-forever
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func processMessage(d amqp.Delivery) (bool, error) {
	var body map[string]interface{}

	err := json.Unmarshal(d.Body, &body)
	if err != nil {
		return false, fmt.Errorf("Failed to read message body: %v", err)
	}

	return processJSONBody(body);
}

func processJSONBody(body map[string]interface{}) (bool, error) {
	log.Printf("Received a message. FileId: %s, EventId: %x", body["FileId"], int(body["EventId"].(float64)))

	path, err := getPathFromTimestamp(body["Timestamp"].(string), body["FileId"].(string))
	if err != nil {
		return false, fmt.Errorf("Unable to generate path from timestamp: %v", err)
	}

	args := transactionservice.Args{}
	args.Path = path

	eventID := int(body["EventId"].(float64))

	if eventID == AnalysisReportID {
		args.WriteAnalysisReport(body["AnalysisReport"].(string))
	} else {
		args.WriteTransactionEvent(body)
	}

	return false, nil
}

func getPathFromTimestamp(timestamp, fileID string) (string, error) {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return "", err
	}

	path := fmt.Sprintf("%s/%d/%d/%d/%d/%s", rootPath, t.Year(), t.Month(), t.Day(), t.Hour(), fileID)

	return path, nil
}