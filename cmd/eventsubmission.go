package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"net/http"
	"net/url"

	transactionservice "github.com/filetrust/event-submission-service/pkg"
	"github.com/filetrust/event-submission-service/pkg/comms"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/streadway/amqp"
)

const (
	ok               = "ok"
	jsonerr          = "json_error"
	timestamperr     = "timestamp_error"
	writeerr         = "write_error"
	analysisReportID = 112
)

var (
	exchange   = "adaptation-exchange"
	routingKey = "transaction-event"
	queueName  = "transaction-event-queue"

	procTime = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "gw_eventsubmission_message_processing_time_millisecond",
			Help:    "Time taken to process queue message",
			Buckets: []float64{5, 10, 100, 250, 500, 1000},
		},
	)

	msgTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gw_eventsubmission_messages_consumed_total",
			Help: "Number of messages consumed from Rabbit",
		},
		[]string{"status"},
	)

	rootPath                      = os.Getenv("TRANSACTION_STORE_ROOT_PATH")
	metricsPort                   = os.Getenv("METRICS_PORT")
	transactionEventQueueHostname = os.Getenv("TRANSACTION_EVENT_QUEUE_HOSTNAME")
	transactionEventQueuePort     = os.Getenv("TRANSACTION_EVENT_QUEUE_PORT")
	messagebrokeruser             = os.Getenv("MESSAGE_BROKER_USER")
	messagebrokerpassword         = os.Getenv("MESSAGE_BROKER_PASSWORD")
)

func main() {
	if rootPath == "" {
		log.Fatalf("init failed: TRANSACTION_STORE_ROOT_PATH")
	}

	if metricsPort == "" {
		log.Fatalf("init failed METRICS_PORT")
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

	amqpURL := url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(messagebrokeruser, messagebrokerpassword),
		Host:   fmt.Sprintf("%s:%s", transactionEventQueueHostname, transactionEventQueuePort),
		Path:   "/",
	}

	forever := make(chan bool)

	fmt.Println("Connecting to ", amqpURL.Host)

	conn := comms.NewConnection("event-submission-consumer", exchange, routingKey, []string{queueName}, amqpURL)
	if err := conn.Connect(); err != nil {
		failOnError(err, "Failed to connect to RabbitMQ")
	}

	if err := conn.BindQueue(); err != nil {
		failOnError(err, "Failed to bind queue")
	}

	deliveries, err := conn.Consume()
	if err != nil {
		failOnError(err, "Failed to register a consumer")
	}

	for q, d := range deliveries {
		go conn.HandleConsumedDeliveries(q, d, messageHandler)
	}

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(fmt.Sprintf(":%v", metricsPort), nil)
	}()

	log.Printf("[*] Waiting for messages. To exit press CTRL+C")
	<-forever
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func messageHandler(c comms.Connection, q string, deliveries <-chan amqp.Delivery) {
	for d := range deliveries {
		processMessage(d)
	}
}

func processMessage(d amqp.Delivery) (bool, error) {
	defer func(start time.Time) {
		procTime.Observe(float64(time.Since(start).Milliseconds()))
	}(time.Now())

	var body map[string]interface{}

	err := json.Unmarshal(d.Body, &body)
	if err != nil {
		msgTotal.WithLabelValues(jsonerr).Inc()
		return false, fmt.Errorf("Failed to read message body: %v", err)
	}

	return processJSONBody(body)
}

func processJSONBody(body map[string]interface{}) (bool, error) {
	log.Printf("Received a message. FileId: %s, EventId: %x", body["FileId"], int(body["EventId"].(float64)))

	path, err := getPathFromTimestamp(body["Timestamp"].(string), body["FileId"].(string))
	if err != nil {
		msgTotal.WithLabelValues(timestamperr).Inc()
		return false, fmt.Errorf("Unable to generate path from timestamp: %v", err)
	}

	args := transactionservice.Args{}
	args.Path = path

	eventID := int(body["EventId"].(float64))

	if eventID == analysisReportID {
		err := args.WriteAnalysisReport(body["AnalysisReport"].(string))

		if err != nil {
			msgTotal.WithLabelValues(writeerr).Inc()
			return true, err
		}

	} else {
		err := args.WriteTransactionEvent(body)

		if err != nil {
			msgTotal.WithLabelValues(writeerr).Inc()
			return true, err
		}
	}

	msgTotal.WithLabelValues(ok).Inc()
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
