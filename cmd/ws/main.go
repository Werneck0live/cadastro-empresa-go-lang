package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
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

type wsConfig struct {
	Addr         string // ex.: ":8090"
	RabbitURI    string
	RabbitQ      string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func loadWSConfig(cfg *config.Config) wsConfig {
	addr := os.Getenv("WS_ADDR")
	if addr == "" {
		addr = ":8090"
	}
	return wsConfig{
		Addr:         addr,
		RabbitURI:    cfg.RabbitURI,
		RabbitQ:      cfg.RabbitQueue,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Ajuste CORS conforme necessário
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	cfg := config.Load()
	_ = config.InitLogger(cfg.LogLevel)
	log := slog.Default().With("svc", "ws")

	// admin task opcional p/ futuro (ex.: -task noop)
	task := flag.String("task", "", "admin task (optional)")
	flag.Parse()
	if *task != "" {
		log.Info("admin_task", "task", *task)
		return
	}

	wscfg := loadWSConfig(cfg)
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
			// aqui você pode normalizar/enriquecer msg, se quiser
			msg := strings.TrimSpace(string(d.Body))
			hub.Broadcast([]byte(msg))
			// auto-ack já estava true; se mudar, chame d.Ack(false)
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

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	_ = srv.Shutdown(ctx)
	hub.Stop()

	log.Info("stopped")
}

func startRabbitConsumer(c wsConfig, log *slog.Logger) (*amqp.Connection, *amqp.Channel, <-chan amqp.Delivery, error) {
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
		c.RabbitQ, // <-- e aqui idem: use c.RabbitQ
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
		c.RabbitQ,
		"ws-consumer",
		true, false, false, false, nil,
	)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, nil, nil, err
	}
	log.Info("rabbit_consumer_started", "queue", c.RabbitQ)
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

	// writer
	go func() {
		defer func() { _ = conn.Close() }()
		for msg := range client.Send {
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				break
			}
		}
	}()

	// reader (descarta frames do cliente; usado só p/ detectar fechamento)
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

// Delegar Flush (para streaming) se o writer de baixo tiver
func (w *statusRW) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Delegar Hijack (necessário para websocket)
func (w *statusRW) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying writer does not support hijacking")
	}
	return h.Hijack()
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Se é upgrade para websocket, não embrulhe o ResponseWriter!
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
