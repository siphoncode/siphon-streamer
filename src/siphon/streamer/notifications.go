// Note that this borrows heavily from the simple consumer in the amqp package
// (the consumer is lightly customised: the exchange type is 'fanout')

package streamer

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/streadway/amqp"
)

var (
	amqpURI = "amqp://" + os.Getenv("RABBITMQ_HOST") + ":" +
		os.Getenv("RABBITMQ_PORT")
	amqpExchange     = "siphon.apps.notifications"
	amqpExchangeType = "fanout"
	amqpConsumerTag  = "siphon-streamer"
	// Note that since the exchangeType is fanout, any queue name may be
	// utilised by producers (will probably be useful to have multiple queues
	// in the future if we have large amounts of traffic)
	amqpQueue = "siphon.apps.notifications-queue"
)

type consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	tag     string
	done    chan error
}

func newConsumer(uri string, exchange string, exchangeType string,
	queueName string, ctag string) (*consumer, <-chan amqp.Delivery, error) {
	c := &consumer{
		conn:    nil,
		channel: nil,
		tag:     ctag,
		done:    make(chan error),
	}

	var err error

	log.Printf("Dialing %s", amqpURI)
	c.conn, err = amqp.Dial(amqpURI)
	if err != nil {
		return nil, nil, fmt.Errorf("Dial: %s", err)
	}

	go func() {
		fmt.Printf("Closing: %s", <-c.conn.NotifyClose(make(chan *amqp.Error)))
	}()

	log.Printf("Got connection, getting channel")
	c.channel, err = c.conn.Channel()
	if err != nil {
		return nil, nil, fmt.Errorf("Channel: %s", err)
	}

	log.Printf("Got channel, declaring exchange (%q)", exchange)
	if err = c.channel.ExchangeDeclare(
		exchange,     // name of the exchange
		exchangeType, // type
		true,         // durable
		false,        // delete when complete
		false,        // internal
		false,        // noWait
		nil,          // arguments
	); err != nil {
		return nil, nil, fmt.Errorf("Exchange declare: %s", err)
	}

	log.Printf("Declared exchange, declaring queue %q", queueName)
	queue, err := c.channel.QueueDeclare(
		queueName, // name of the queue
		true,      // durable
		false,     // delete when usused
		false,     // exclusive
		false,     // noWait
		nil,       // arguments
	)
	if err != nil {
		return nil, nil, fmt.Errorf("Queue declare: %s", err)
	}

	log.Printf("Declared queue (%q %d messages, %d consumers), binding to "+
		"exchange", queue.Name, queue.Messages, queue.Consumers)

	if err = c.channel.QueueBind(
		queue.Name, // name of the queue
		"bind",     // bindingKey (ignored by fanout type)
		exchange,   // sourceExchange
		false,      // noWait
		nil,        // arguments
	); err != nil {
		return nil, nil, fmt.Errorf("Queue bind: %s", err)
	}

	log.Printf("Queue bound to exchange, starting consuming "+
		"(consumer tag %q)", c.tag)
	deliveries, err := c.channel.Consume(
		queue.Name, // name
		c.tag,      // consumerTag,
		true,       // acknowledge
		false,      // exclusive
		false,      // noLocal
		false,      // noWait
		nil,        // arguments
	)
	if err != nil {
		return nil, nil, fmt.Errorf("Queue consume: %s", err)
	}

	return c, deliveries, nil
}

type notification struct {
	AppID            string `json:"app_id"`
	UserID           string `json:"user_id"`
	NotificationType string `json:"type"`
}

// Note: deliveries <-chan ampq.Delivery means a channel that can only receive
// Deliveries
func handle(connErrors chan *amqp.Error, deliveries <-chan amqp.Delivery) {
	for {
		select {
		case err := <-connErrors:
			log.Printf("Connection closed with err: %v", err)
			runNotificationListener()
		case delivery, ok := <-deliveries:
			if !ok {
				break
			}
			var n notification
			err := json.Unmarshal(delivery.Body, &n)
			if err != nil {
				log.Printf("Error unmarshalling delivery: %v", err)
				break
			}

			if n.NotificationType == "" || n.AppID == "" {
				log.Printf("Bad notification received")
				break
			}
			m := message{appID: n.AppID, userID: n.UserID, payload: delivery.Body}
			d.dispatchNotification(m)
		}
	}
}

func runNotificationListener() {
	log.Printf("Running notification listener...")
	c, delivery, err := newConsumer(amqpURI, amqpExchange, amqpExchangeType,
		amqpQueue, amqpConsumerTag)
	if err != nil {
		log.Printf("Error starting consumer: %v", err)
		log.Printf("Trying again in 10 seconds")
		time.Sleep(10 * time.Second)
		runNotificationListener()
	}

	// Register channel for errors resulting in closed connections
	connErrors := make(chan *amqp.Error)
	c.conn.NotifyClose(connErrors)

	handle(connErrors, delivery)
}
