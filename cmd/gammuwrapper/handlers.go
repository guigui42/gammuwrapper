package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/rs/zerolog/log"
)

func (s *Server) MountHandlers() {
	s.Router.Use(middleware.Logger)
	s.Router.Use(middleware.Heartbeat("/ping"))

	s.Router.Post("/sendsms", s.SendSMS)
	s.Router.Get("/health", s.HTTPHealth)
	s.Router.Get("/health/modem", s.ModemHealth)
	s.Router.Get("/health/network", s.NetworkHealth)
}

func (s *Server) SendSMS(w http.ResponseWriter, r *http.Request) {
	var sms SMS
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	if err := decoder.Decode(&sms); err != nil {
		log.Warn().Err(err).Msg("invalid SMS request")
		http.Error(w, "invalid JSON request", http.StatusBadRequest)
		return
	}
	if err := ensureJSONEnd(decoder); err != nil {
		log.Warn().Err(err).Msg("invalid SMS request")
		http.Error(w, "request must contain one JSON object", http.StatusBadRequest)
		return
	}
	if sms.PhoneNumber == "" || sms.Message == "" {
		log.Warn().Msg("SMS request missing required fields")
		http.Error(w, "phone_number and message are required", http.StatusBadRequest)
		return
	}

	if !s.tryAcquireModem() {
		log.Warn().Msg("SMS rejected because modem is busy")
		http.Error(w, "modem is busy", http.StatusTooManyRequests)
		return
	}
	defer s.releaseModem()

	ctx, cancel := context.WithTimeout(r.Context(), s.sendTimeout)
	defer cancel()

	started := time.Now()
	err := s.gammu.SendSMS(ctx, sms)
	duration := time.Since(started)
	if err != nil {
		event := s.logger.Error().
			Err(err).
			Int("message_bytes", len([]byte(sms.Message))).
			Dur("duration", duration)
		if errors.Is(err, context.DeadlineExceeded) {
			event.Msg("SMS send timed out")
			http.Error(w, "SMS send timed out", http.StatusGatewayTimeout)
			return
		}

		event.Msg("SMS send failed")
		http.Error(w, "Gammu failed to send SMS", http.StatusBadGateway)
		return
	}

	s.logger.Info().
		Int("message_bytes", len([]byte(sms.Message))).
		Dur("duration", duration).
		Msg("SMS sent")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("SMS sent"))
}

func (s *Server) HTTPHealth(w http.ResponseWriter, _ *http.Request) {
	writeHealth(w, http.StatusOK, "http", "ok", "")
}

func (s *Server) ModemHealth(w http.ResponseWriter, r *http.Request) {
	s.runDiagnostic(w, r, "modem", s.gammu.DevicePresent)
}

func (s *Server) NetworkHealth(w http.ResponseWriter, r *http.Request) {
	s.runDiagnostic(w, r, "network", s.gammu.NetworkReady)
}

func (s *Server) runDiagnostic(
	w http.ResponseWriter,
	r *http.Request,
	component string,
	check func(context.Context) error,
) {
	if !s.tryAcquireModem() {
		writeHealth(w, http.StatusServiceUnavailable, component, "busy", "modem is in use")
		return
	}
	defer s.releaseModem()

	ctx, cancel := context.WithTimeout(r.Context(), s.sendTimeout)
	defer cancel()
	if err := check(ctx); err != nil {
		status := "unavailable"
		message := fmt.Sprintf("%s check failed", component)
		if errors.Is(err, context.DeadlineExceeded) {
			status = "timeout"
			message = fmt.Sprintf("%s check timed out", component)
		}
		s.logger.Warn().Err(err).Str("component", component).Msg("health check failed")
		writeHealth(w, http.StatusServiceUnavailable, component, status, message)
		return
	}

	writeHealth(w, http.StatusOK, component, "ok", "")
}

func (s *Server) tryAcquireModem() bool {
	select {
	case <-s.modemSlot:
		return true
	default:
		return false
	}
}

func (s *Server) releaseModem() {
	s.modemSlot <- struct{}{}
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func writeHealth(w http.ResponseWriter, code int, component, status, message string) {
	response := map[string]string{
		"component": component,
		"status":    status,
	}
	if message != "" {
		response["message"] = message
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(response)
}
