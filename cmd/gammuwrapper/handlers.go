package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/middleware"
)

type healthResponse struct {
	Component string `json:"component"`
	Status    string `json:"status"`
}

func (s *Server) MountHandlers() {
	// Mount all Middleware here
	//s.Router.Use(LoggerMiddleware(s.logger))
	s.Router.Use(middleware.Logger)
	//s.Router.Use(middleware.Recoverer)
	//s.Router.Use(middleware.CleanPath)
	s.Router.Use(middleware.Heartbeat("/ping"))

	// Mount all handlers here
	s.Router.Post("/sendsms", s.AddSMSToQueue)
	s.Router.Get("/health", s.Health)
	s.Router.Get("/health/modem", s.ModemHealth)
	s.Router.Get("/health/network", s.NetworkHealth)
}

func (s *Server) Health(w http.ResponseWriter, _ *http.Request) {
	writeHealth(w, http.StatusOK, "http", "ok")
}

func (s *Server) ModemHealth(w http.ResponseWriter, r *http.Request) {
	status, err := s.runDiagnostic(r.Context(), s.gammu.DevicePresent)
	if err != nil {
		writeHealth(w, http.StatusServiceUnavailable, "modem", status)
		return
	}
	writeHealth(w, http.StatusOK, "modem", "ok")
}

func (s *Server) NetworkHealth(w http.ResponseWriter, r *http.Request) {
	status, err := s.runDiagnostic(r.Context(), func(ctx context.Context) error {
		ready, err := s.gammu.NetworkReady(ctx)
		if err != nil {
			return err
		}
		if !ready {
			return errNetworkUnregistered
		}
		return nil
	})
	if err != nil {
		if status == "unavailable" {
			status = "unregistered"
		}
		writeHealth(w, http.StatusServiceUnavailable, "network", status)
		return
	}
	writeHealth(w, http.StatusOK, "network", "ok")
}

var errNetworkUnregistered = errors.New("network unregistered")

func (s *Server) runDiagnostic(requestCtx context.Context, operation func(context.Context) error) (string, error) {
	diagnosticCtx, cancel := context.WithTimeout(requestCtx, s.diagnosticTimeout)
	defer cancel()

	if err := s.modemGate.Acquire(diagnosticCtx); err != nil {
		return "busy", err
	}
	defer s.modemGate.Release()

	err := operation(diagnosticCtx)
	if err == nil {
		return "ok", nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(diagnosticCtx.Err(), context.DeadlineExceeded) {
		return "timeout", err
	}
	return "unavailable", err
}

func writeHealth(w http.ResponseWriter, statusCode int, component, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(healthResponse{Component: component, Status: status})
}
