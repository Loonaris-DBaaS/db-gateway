package gateway

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type Config struct {
	Address           string
	ControlPlaneURL   string
	GatewaySecret     string
	BackendDBUser     string
	BackendDBPassword string
	ShutdownTimeout   time.Duration
}

type Server struct {
	config     Config
	numClients atomic.Int64
	httpClient *http.Client
	routeCache sync.Map
	sfGroup    singleflightGroup
	listener   net.Listener
	ready      chan struct{}
}

func NewServer(cfg Config) *Server {
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 30 * time.Second
	}
	return &Server{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		ready: make(chan struct{}),
	}
}

func (s *Server) ListenAndServe() error {
	listener, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return err
	}
	s.listener = listener

	fmt.Printf("Gateway listening on %s\n", s.config.Address)
	fmt.Printf("Control plane: %s\n", s.config.ControlPlaneURL)
	close(s.ready)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("shutting down gracefully...")
		s.listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-sigCh:
				fmt.Println("waiting for existing connections to close...")
				shutdownStart := time.Now()
				for s.numClients.Load() > 0 {
					if time.Since(shutdownStart) > s.config.ShutdownTimeout {
						fmt.Println("shutdown timeout exceeded, forcing exit")
						break
					}
					time.Sleep(100 * time.Millisecond)
				}
				fmt.Println("all connections closed")
				return nil
			default:
				fmt.Printf("accept failed: %v\n", err)
				return err
			}
		}

		s.numClients.Add(1)
		go s.handleConn(conn)
	}
}

func (s *Server) Ready() <-chan struct{} {
	return s.ready
}