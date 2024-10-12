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
	Router *chi.Mux
	queue  *BQueue
	worker *Worker
	logger *zerolog.Logger
	// Db, config can be added here
}

type SMS struct {
	PhoneNumber string
	Message     string
}

var smsServer *Server

func main() {
	// LOGGING
	log.Logger = log.With().Caller().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05", NoColor: false})
	// log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05", NoColor: false})
	loglevel, _ := zerolog.ParseLevel("INFO")
	zerolog.SetGlobalLevel(loglevel)

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
		shutdownCtx, _ := context.WithTimeout(serverCtx, 30*time.Second)

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

	// Run the server
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Error().Err(err).Msg("")
	}

	// Wait for server context to be stopped
	<-serverCtx.Done()
}

// AddSMSToQueue api Handler
func AddSMSToQueue(w http.ResponseWriter, r *http.Request) {
	if smsServer == nil {
		log.Error().Msg("Server is not initialized")
		w.Write([]byte("Server is not initialized"))
		return
	}
	if smsServer.queue == nil {
		log.Error().Msg("Queue is not initialized")
		w.Write([]byte("Queue is not initialized"))
		return
	}
	err := smsServer.queue.Enqueue(SMS{PhoneNumber: "0033619651893", Message: "Hello World!"})
	if err != nil {
		log.Error().Err(err).Msgf("Error sending SMS: %v", err)
		return
	}
	w.Write([]byte("SMS added to queue"))

}

func CreateNewServer() *Server {
	s := &Server{}
	s.Router = chi.NewRouter()
	s.queue = NewQueue(conf.Conf.SMSQueueMaxSize)

	// Defines a queue worker, which will execute our queue.
	s.worker = NewWorker(s.queue)
	return s
}
