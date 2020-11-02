package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/filetrust/event-submission-service/pkg"
	"github.com/streadway/amqp"
)

var (
	exchange   = "adaptation-exchange"
	routingKey = "transaction-event"
	queueName  = "transaction-event-queue"

	amqpURL     = os.Getenv("AMQP_URL")
	accountName = os.Getenv("ACCOUNT_NAME")
	accountKey  = os.Getenv("ACCOUNT_KEY")
)

const AnalysisReportID = 112

func main() {
	if amqpURL == "" || accountName == "" || accountKey == "" {
		log.Fatalf("init failed: AMQP_URL, ACCOUNT_NAME or ACCOUNT_KEY environment variables not set")
	}

	conn, err := amqp.Dial(amqpURL)
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

	log.Printf("Received a message. FileId: %s, EventId: %x", body["FileId"], int(body["EventId"].(float64)))

	args := uploader.UploaderArgs{
		AccountName: accountName,
		AccountKey:  accountKey,
	}

	err = args.GetPipeline()
	if err != nil {
		return true, fmt.Errorf("Failed to create pipeline to share: %v", err)
	}

	err = args.GetPaths(body["Timestamp"].(string), body["FileId"].(string))
	if err != nil {
		return true, fmt.Errorf("Failed to get file paths: %v", err)
	}

	eventID := int(body["EventId"].(float64))

	if eventID == AnalysisReportID {
		args.UploadAnalysisReport(body["AnalysisReport"].(string))
	} else {
		args.UploadTransactionEvent(body)
	}

	return false, nil
}
