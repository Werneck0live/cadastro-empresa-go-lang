//go:build integration
// +build integration

package broker

/*
	Para rodar: go test -tags=integration -v ./internal/broker -run TestRabbitMQ_PublishAndConsume -count=1

	obs: Rodar todos os de integração: go test -tags=integration -v ./... -count=1
*/

import (
	"context"
	"fmt"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Sobe RabbitMQ real, publica com o Publisher e consome pela lib para validar a mensagem
func TestRabbitMQ_PublishAndConsume(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Sobe container do RabbitMQ
	req := tc.ContainerRequest{
		Image:        "rabbitmq:3.13",
		ExposedPorts: []string{"5672/tcp"},
		WaitingFor:   wait.ForListeningPort("5672/tcp").WithStartupTimeout(60 * time.Second),
	}
	c, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: req, Started: true})
	if err != nil {
		t.Fatalf("start rabbit: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	host, err := c.Host(ctx)
	if err != nil {
		t.Fatalf("host: %v", err)
	}
	port, err := c.MappedPort(ctx, "5672/tcp")
	if err != nil {
		t.Fatalf("port: %v", err)
	}

	uri := fmt.Sprintf("amqp://guest:guest@%s:%s/", host, port.Port())
	queue := "companies_test"

	// Publisher
	pub, err := NewPublisher(uri, queue)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	t.Cleanup(func() { _ = pub.Close() })

	// Consumer direto pela lib amqp
	conn, err := amqp.Dial(uri)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("channel: %v", err)
	}
	t.Cleanup(func() { _ = ch.Close() })

	// Garante a fila (idempotente)
	_, err = ch.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		t.Fatalf("queue declare: %v", err)
	}

	msgs, err := ch.Consume(queue, "", true, false, false, false, nil)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}

	// Publica
	body := "hello-integration"
	headers := amqp.Table{"k": "v"}
	if err := pub.Publish(ctx, body, headers); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Aguarda receber (com timeout)
	select {
	case m := <-msgs:
		if string(m.Body) != body {
			t.Fatalf("body mismatch: got=%q want=%q", string(m.Body), body)
		}
		// (opcional) checar cabeçalhos
		if m.Headers["k"] != "v" {
			t.Fatalf("header mismatch: %#v", m.Headers)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout esperando mensagem")
	}
}
