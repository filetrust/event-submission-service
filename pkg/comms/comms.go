package comms

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/streadway/amqp"
)

//Connection is the connection created
type Connection struct {
	name       string
	amqpURL    url.URL
	conn       *amqp.Connection
	channel    *amqp.Channel
	exchange   string
	queues     []string
	routingKey string
	err        chan error
}

var (
	connectionPool = make(map[string]*Connection)
)

//NewConnection returns the new connection object
func NewConnection(name, exchange, routingKey string, queues []string, amqpURL url.URL) *Connection {
	if c, ok := connectionPool[name]; ok {
		return c
	}
	c := &Connection{
		exchange:   exchange,
		queues:     queues,
		routingKey: routingKey,
		amqpURL:    amqpURL,
		err:        make(chan error),
	}
	connectionPool[name] = c
	return c
}

//GetConnection returns the connection which was instantiated
func GetConnection(name string) *Connection {
	return connectionPool[name]
}

func (c *Connection) Connect() error {
	var err error
	c.conn, err = amqp.Dial(c.amqpURL.String())
	if err != nil {
		return fmt.Errorf("Error in creating rabbitmq connection with %s : %s", c.amqpURL.Host, err.Error())
	}
	go func() {
		<-c.conn.NotifyClose(make(chan *amqp.Error)) //Listen to NotifyClose
		c.err <- errors.New("Connection Closed")
	}()
	c.channel, err = c.conn.Channel()
	if err != nil {
		return fmt.Errorf("Channel: %s", err)
	}
	if err := c.channel.ExchangeDeclare(
		c.exchange, // name
		"direct",   // type
		true,       // durable
		false,      // auto-deleted
		false,      // internal
		false,      // noWait
		nil,        // arguments
	); err != nil {
		return fmt.Errorf("Error in Exchange Declare: %s", err)
	}
	return nil
}

func (c *Connection) BindQueue() error {
	for _, q := range c.queues {
		if _, err := c.channel.QueueDeclare(q, true, false, false, false, nil); err != nil {
			return fmt.Errorf("error in declaring the queue %s", err)
		}
		if err := c.channel.QueueBind(q, c.routingKey, c.exchange, false, nil); err != nil {
			return fmt.Errorf("Queue  Bind error: %s", err)
		}
	}
	return nil
}

//Reconnect reconnects the connection
func (c *Connection) Reconnect() error {
	if err := c.Connect(); err != nil {
		return err
	}
	if err := c.BindQueue(); err != nil {
		return err
	}
	return nil
}

//Consume consumes the messages from the queues and passes it as map of chan of amqp.Delivery
func (c *Connection) Consume() (map[string]<-chan amqp.Delivery, error) {
	m := make(map[string]<-chan amqp.Delivery)
	for _, q := range c.queues {
		deliveries, err := c.channel.Consume(q, "", true, false, false, false, nil)
		if err != nil {
			return nil, err
		}
		m[q] = deliveries
	}
	return m, nil
}

//HandleConsumedDeliveries handles the consumed deliveries from the queues. Should be called only for a consumer connection
func (c *Connection) HandleConsumedDeliveries(q string, delivery <-chan amqp.Delivery, fn func(Connection, string, <-chan amqp.Delivery)) {
	for {
		go fn(*c, q, delivery)
		if err := <-c.err; err != nil {
			c.Reconnect()
			deliveries, err := c.Consume()
			if err != nil {
				panic(err) //raising panic if consume fails even after reconnecting
			}
			delivery = deliveries[q]
		}
	}
}
