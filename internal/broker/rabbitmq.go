package broker

import (
	"context"
	"errors"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	conn  *amqp.Connection
	ch    *amqp.Channel
	queue string
}

func NewPublisher(uri, queue string) (*Publisher, error) {
	conn, err := amqp.Dial(uri)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	// Garante que a fila exista (durável)
	_, err = ch.QueueDeclare(
		queue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	return &Publisher{conn: conn, ch: ch, queue: queue}, nil
}

func (p *Publisher) Publish(ctx context.Context, body string, headers amqp.Table) error {
	if ctx == nil {
		c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ctx = c
	}
	return p.ch.PublishWithContext(
		ctx,
		"",      // default exchange
		p.queue, // routing key = nome da fila
		false,   // mandatory
		false,   // immediate
		amqp.Publishing{
			ContentType:  "text/plain",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			Body:         []byte(body),
			Headers:      headers,
		},
	)
}

func (p *Publisher) Close() error {
	var errCh, errConn error
	if p.ch != nil {
		errCh = p.ch.Close()
	}
	if p.conn != nil {
		errConn = p.conn.Close()
	}

	return errors.Join(errCh, errConn)
}
