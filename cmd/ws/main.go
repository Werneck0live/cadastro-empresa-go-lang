package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/Werneck0live/cadastro-empresa/internal/config"
	"github.com/Werneck0live/cadastro-empresa/internal/ws"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Ajuste CORS conforme necessário
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {

	wscfg := config.LoadWSConfig()

	_ = config.InitLogger(wscfg.LogLevel)
	log := slog.Default().With("svc", "ws")
	hub := ws.NewHub(log)
	go hub.Run()

	// Conecta no Rabbit e começa a consumir
	conn, ch, deliveries, err := startRabbitConsumer(wscfg, log)
	if err != nil {
		log.Error("rabbit_consumer_start_error", "err", err)
		os.Exit(1)
	}
	defer func() {
		_ = ch.Close()
		_ = conn.Close()
	}()

	// encaminha mensagens do Rabbit para o hub
	go func() {
		for d := range deliveries {
			hub.Broadcast(d.Body)
		}
		log.Warn("deliveries_channel_closed")
	}()

	// HTTP: /ws e /healthz
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWS(hub, w, r, log)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	srv := &http.Server{
		Addr:              wscfg.Addr,
		Handler:           logMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// O servidor é inicializado e começa a escutar na porta configurada
	go func() {
		log.Info("ws_listen", "addr", wscfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http_server_error", "err", err)
			os.Exit(1)
		}
	}()

	// graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), wscfg.ShutdownTimeout)
	defer cancel()
	_ = srv.Shutdown(ctx)
	hub.Stop()

	log.Info("stopped")
}

func startRabbitConsumer(c *config.WSConfig, log *slog.Logger) (*amqp.Connection, *amqp.Channel, <-chan amqp.Delivery, error) {
	conn, err := amqp.Dial(c.RabbitURI) // <-- aqui tinha "c RabbitURI"
	if err != nil {
		return nil, nil, nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, nil, err
	}

	if _, err = ch.QueueDeclare(
		c.RabbitQueue, // <-- e aqui idem: use c.RabbitQ
		true, false, false, false, nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, nil, nil, err
	}

	if err := ch.Qos(50, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, nil, nil, err
	}

	deliveries, err := ch.Consume(
		c.RabbitQueue,
		"ws-consumer",
		true, false, false, false, nil,
	)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, nil, nil, err
	}
	log.Info("rabbit_consumer_started", "queue", c.RabbitQueue)
	return conn, ch, deliveries, nil
}

func handleWS(hub *ws.Hub, w http.ResponseWriter, r *http.Request, log *slog.Logger) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("ws_upgrade_error", "err", err)
		return
	}

	client := &ws.Client{Send: make(chan []byte, 256)}
	hub.Register(client)
	log.Info("ws_client_connected", "id", client.ID)

	// writer
	// Envia mensagens para o WebSocket do cliente sempre que uma nova mensagem é recebida pelo hub
	go func() {
		defer func() { _ = conn.Close() }()
		for msg := range client.Send {
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				break
			}
		}
	}()

	// Detecta o fechamento do WebSocket e lida com a recepção de mensagens
	go func() {
		defer func() {
			hub.Unregister(client)
			_ = conn.Close()
		}()
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}

type statusRW struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusRW) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusRW) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// Loga as requisições HTTP, incluindo o método, status, n. de bytes e ttl
func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Se é upgrade para websocket, não embrulha o ResponseWriter!
		if strings.EqualFold(r.Header.Get("Connection"), "Upgrade") ||
			strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
			r.URL.Path == "/ws" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		srw := &statusRW{ResponseWriter: w}
		next.ServeHTTP(srw, r)
		slog.Info("http_request",
			"method", r.Method, "path", r.URL.Path,
			"status", srw.status, "bytes", srw.bytes,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}
