package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/guigui42/gammuwrapper/conf"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Server struct {
	Router            *chi.Mux
	queue             *BQueue
	worker            *Worker
	logger            *zerolog.Logger
	gammu             GammuOperations
	modemGate         *ModemGate
	diagnosticTimeout time.Duration
}

type SMS struct {
	PhoneNumber string `json:"phone_number"`
	Message     string `json:"message"`
}

var smsServer *Server

func main() {
	// LOGGING
	log.Logger = log.With().Caller().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05", NoColor: false})
	// log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05", NoColor: false})
	loglevel, _ := zerolog.ParseLevel("INFO")
	zerolog.SetGlobalLevel(loglevel)

	// CONFIGURATION
	// Load environment variables
	if err := conf.LoadConf(); err != nil {
		log.Fatal().Err(err).Msg("invalid configuration")
	}

	smsServer = CreateNewServer()
	smsServer.logger = &log.Logger
	smsServer.MountHandlers()

	// Execute SMS jobs in queue in the background.
	go func() {
		smsServer.worker.WaitForSMS()
	}()

	// The HTTP Server
	server := &http.Server{Addr: fmt.Sprintf("0.0.0.0:%v", conf.Conf.Port), Handler: smsServer.Router}

	// Server run context
	serverCtx, serverStopCtx := context.WithCancel(context.Background())

	// Listen for syscall signals for process to interrupt/quit
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sig

		// Shutdown signal with grace period of 30 seconds
		shutdownCtx, shutdownCancel := context.WithTimeout(serverCtx, 30*time.Second)
		defer shutdownCancel()

		go func() {
			<-shutdownCtx.Done()
			if shutdownCtx.Err() == context.DeadlineExceeded {
				log.Error().Msg("graceful shutdown timed out.. forcing exit.")
			}
		}()

		// Trigger graceful shutdown
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
		serverStopCtx()
	}()

	log.Info().Msgf("Server 2 started on port %v", conf.Conf.Port)
	// Run the server
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Error().Err(err).Msg("")
	}

	// Wait for server context to be stopped
	<-serverCtx.Done()
}

// AddSMSToQueue api Handler
func (s *Server) AddSMSToQueue(w http.ResponseWriter, r *http.Request) {
	// Read body
	b, err := io.ReadAll(r.Body)
	defer func() {
		if closeErr := r.Body.Close(); closeErr != nil {
			log.Error().Msg("Error closing request body")
		}
	}()
	if err != nil {
		log.Error().Msg("Error reading request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TOFIX: remove new lines from body to avoid json decoding error
	body := strings.ReplaceAll(string(b), "\n", "")
	// unmarschal the request body
	var sms SMS
	err = json.Unmarshal([]byte(body), &sms)
	if err != nil {
		log.Error().Msg("Error decoding request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if sms.PhoneNumber == "" || sms.Message == "" {
		log.Error().Msg("Missing required fields")
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	err = s.queue.Enqueue(sms)
	if err != nil {
		if err == ErrQueueFull {
			http.Error(w, "SMS queue is full", http.StatusServiceUnavailable)
			return
		}
		log.Error().Err(err).Msg("Error adding SMS to queue")
		http.Error(w, "Unable to add SMS to queue", http.StatusInternalServerError)
		return
	}
	if _, err := w.Write([]byte("SMS added to queue")); err != nil {
		log.Error().Msg("Error writing response")
	}
}

func CreateNewServer() *Server {
	gammu := NewGammuClient(conf.Conf.GammuConf, ExecCommandRunner{})
	return NewServer(
		conf.Conf.SMSQueueMaxSize,
		gammu,
		time.Duration(conf.Conf.GammuSendTimeoutSeconds)*time.Second,
		time.Duration(conf.Conf.GammuDiagnosticTimeoutSeconds)*time.Second,
	)
}

func NewServer(queueSize int, gammu GammuOperations, sendTimeout, diagnosticTimeout time.Duration) *Server {
	queue := NewQueue(queueSize)
	gate := NewModemGate()
	return &Server{
		Router:            chi.NewRouter(),
		queue:             queue,
		worker:            NewWorker(queue, gammu, gate, sendTimeout),
		gammu:             gammu,
		modemGate:         gate,
		diagnosticTimeout: diagnosticTimeout,
	}
}
