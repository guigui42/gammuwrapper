package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/guigui42/gammuwrapper/conf"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Server struct {
	Router      *chi.Mux
	gammu       Gammu
	logger      *zerolog.Logger
	sendTimeout time.Duration
	modemSlot   chan struct{}
}

type SMS struct {
	PhoneNumber string `json:"phone_number"`
	Message     string `json:"message"`
}

func main() {
	// LOGGING
	log.Logger = log.With().Caller().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05", NoColor: false})
	// log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05", NoColor: false})
	loglevel, _ := zerolog.ParseLevel("INFO")
	zerolog.SetGlobalLevel(loglevel)

	// CONFIGURATION
	// Load environment variables
	if err := conf.LoadConf(); err != nil {
		log.Fatal().Err(err).Msg("error loading configuration")
	}

	smsServer := CreateNewServer()
	smsServer.logger = &log.Logger
	smsServer.MountHandlers()

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

func CreateNewServer() *Server {
	return newServer(
		NewGammuClient(conf.Conf.GammuConf, ExecCommandRunner{}),
		time.Duration(conf.Conf.GammuSendTimeoutSeconds)*time.Second,
	)
}

func newServer(gammu Gammu, sendTimeout time.Duration) *Server {
	slot := make(chan struct{}, 1)
	slot <- struct{}{}

	return &Server{
		Router:      chi.NewRouter(),
		gammu:       gammu,
		logger:      &log.Logger,
		sendTimeout: sendTimeout,
		modemSlot:   slot,
	}
}
