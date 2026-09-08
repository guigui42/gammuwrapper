package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

type fakeGammu struct {
	sendSMS       func(context.Context, SMS) error
	devicePresent func(context.Context) error
	networkReady  func(context.Context) (bool, error)
}

func (f fakeGammu) SendSMS(ctx context.Context, sms SMS) error {
	if f.sendSMS == nil {
		return nil
	}
	return f.sendSMS(ctx, sms)
}

func (f fakeGammu) DevicePresent(ctx context.Context) error {
	if f.devicePresent == nil {
		return nil
	}
	return f.devicePresent(ctx)
}

func (f fakeGammu) NetworkReady(ctx context.Context) (bool, error) {
	if f.networkReady == nil {
		return true, nil
	}
	return f.networkReady(ctx)
}

func newTestServer(t *testing.T, queueSize int, gammu GammuOperations) *Server {
	t.Helper()
	server := NewServer(queueSize, gammu, 100*time.Millisecond, 20*time.Millisecond)
	server.MountHandlers()
	return server
}

func startWorker(t *testing.T, server *Server) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		server.worker.WaitForSMS()
	}()
	t.Cleanup(func() {
		server.queue.cancel()
		<-done
	})
}

func executeRequest(req *http.Request, server *Server) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, req)
	return response
}

func smsRequest(t *testing.T, phoneNumber, message string) *http.Request {
	t.Helper()
	body, err := json.Marshal(SMS{PhoneNumber: phoneNumber, Message: message})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, "/sendsms", bytes.NewReader(body))
	require.NoError(t, err)
	return request
}

func assertHealthResponse(t *testing.T, response *httptest.ResponseRecorder, code int, component, status string) {
	t.Helper()
	require.Equal(t, code, response.Code)
	require.Equal(t, "application/json", response.Header().Get("Content-Type"))

	var body healthResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, healthResponse{Component: component, Status: status}, body)
}

func TestAddSMSToQueueAcceptsBeforeDelivery(t *testing.T) {
	server := newTestServer(t, 1, fakeGammu{
		sendSMS: func(context.Context, SMS) error {
			t.Fatal("delivery must not run in the request handler")
			return nil
		},
	})

	response := executeRequest(smsRequest(t, "33600000000", "queued message"), server)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "SMS added to queue", response.Body.String())
	require.Len(t, server.queue.channel, 1)
}

func TestAddSMSToQueueReturnsServiceUnavailableWhenFull(t *testing.T) {
	server := newTestServer(t, 1, fakeGammu{})
	require.NoError(t, server.queue.Enqueue(SMS{PhoneNumber: "first", Message: "first"}))

	response := executeRequest(smsRequest(t, "second", "second"), server)

	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.Equal(t, "SMS queue is full\n", response.Body.String())
}

func TestHealthEndpointContracts(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		gammu     fakeGammu
		code      int
		component string
		status    string
	}{
		{name: "http", path: "/health", code: http.StatusOK, component: "http", status: "ok"},
		{name: "modem present", path: "/health/modem", code: http.StatusOK, component: "modem", status: "ok"},
		{
			name: "modem unavailable", path: "/health/modem",
			gammu: fakeGammu{devicePresent: func(context.Context) error { return errors.New("identify failed") }},
			code:  http.StatusServiceUnavailable, component: "modem", status: "unavailable",
		},
		{name: "network ready", path: "/health/network", code: http.StatusOK, component: "network", status: "ok"},
		{
			name: "network unregistered", path: "/health/network",
			gammu: fakeGammu{networkReady: func(context.Context) (bool, error) { return false, nil }},
			code:  http.StatusServiceUnavailable, component: "network", status: "unregistered",
		},
		{
			name: "network command failure", path: "/health/network",
			gammu: fakeGammu{networkReady: func(context.Context) (bool, error) { return false, errors.New("failed") }},
			code:  http.StatusServiceUnavailable, component: "network", status: "unregistered",
		},
		{
			name: "modem timeout", path: "/health/modem",
			gammu: fakeGammu{devicePresent: func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			}},
			code: http.StatusServiceUnavailable, component: "modem", status: "timeout",
		},
		{
			name: "network timeout", path: "/health/network",
			gammu: fakeGammu{networkReady: func(ctx context.Context) (bool, error) {
				<-ctx.Done()
				return false, ctx.Err()
			}},
			code: http.StatusServiceUnavailable, component: "network", status: "timeout",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newTestServer(t, 1, test.gammu)
			response := executeRequest(httptest.NewRequest(http.MethodGet, test.path, nil), server)
			assertHealthResponse(t, response, test.code, test.component, test.status)
		})
	}
}

func TestDiagnosticReturnsBusyWhileWorkerUsesModem(t *testing.T) {
	sendStarted := make(chan struct{})
	releaseSend := make(chan struct{})
	server := newTestServer(t, 1, fakeGammu{
		sendSMS: func(context.Context, SMS) error {
			close(sendStarted)
			<-releaseSend
			return nil
		},
	})

	startWorker(t, server)
	require.NoError(t, server.queue.Enqueue(SMS{PhoneNumber: "private", Message: "private"}))
	<-sendStarted

	response := executeRequest(httptest.NewRequest(http.MethodGet, "/health/modem", nil), server)
	assertHealthResponse(t, response, http.StatusServiceUnavailable, "modem", "busy")

	close(releaseSend)
}

func TestWorkerWaitsForDiagnosticAndUsesFreshSendTimeout(t *testing.T) {
	diagnosticStarted := make(chan struct{})
	releaseDiagnostic := make(chan struct{})
	sendFinished := make(chan error, 1)
	var diagnosticStartedOnce sync.Once
	server := newTestServer(t, 1, fakeGammu{
		devicePresent: func(context.Context) error {
			diagnosticStartedOnce.Do(func() { close(diagnosticStarted) })
			<-releaseDiagnostic
			return nil
		},
		sendSMS: func(ctx context.Context, _ SMS) error {
			<-ctx.Done()
			sendFinished <- ctx.Err()
			return ctx.Err()
		},
	})
	server.diagnosticTimeout = time.Second
	server.worker.SendTimeout = 10 * time.Millisecond

	diagnosticDone := make(chan struct{})
	diagnosticResponse := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		defer close(diagnosticDone)
		diagnosticResponse <- executeRequest(httptest.NewRequest(http.MethodGet, "/health/modem", nil), server)
	}()
	<-diagnosticStarted

	startWorker(t, server)
	require.NoError(t, server.queue.Enqueue(SMS{PhoneNumber: "private", Message: "private"}))
	select {
	case <-sendFinished:
		t.Fatal("worker sent while diagnostic held the modem gate")
	case <-time.After(20 * time.Millisecond):
	}

	close(releaseDiagnostic)
	<-diagnosticDone
	assertHealthResponse(t, <-diagnosticResponse, http.StatusOK, "modem", "ok")
	require.ErrorIs(t, <-sendFinished, context.DeadlineExceeded)

	response := executeRequest(httptest.NewRequest(http.MethodGet, "/health/modem", nil), server)
	assertHealthResponse(t, response, http.StatusOK, "modem", "ok")
}

func TestWorkerReleasesGateAfterSendFailure(t *testing.T) {
	sendFinished := make(chan struct{})
	server := newTestServer(t, 1, fakeGammu{
		sendSMS: func(context.Context, SMS) error {
			close(sendFinished)
			return errors.New("send failed")
		},
	})

	startWorker(t, server)
	require.NoError(t, server.queue.Enqueue(SMS{PhoneNumber: "private", Message: "private"}))
	<-sendFinished

	response := executeRequest(httptest.NewRequest(http.MethodGet, "/health/modem", nil), server)
	assertHealthResponse(t, response, http.StatusOK, "modem", "ok")
}

func TestDiagnosticReleasesGateAfterFailure(t *testing.T) {
	var mutex sync.Mutex
	calls := 0
	server := newTestServer(t, 1, fakeGammu{
		devicePresent: func(context.Context) error {
			mutex.Lock()
			defer mutex.Unlock()
			calls++
			if calls == 1 {
				return errors.New("first call failed")
			}
			return nil
		},
	})

	first := executeRequest(httptest.NewRequest(http.MethodGet, "/health/modem", nil), server)
	second := executeRequest(httptest.NewRequest(http.MethodGet, "/health/modem", nil), server)

	assertHealthResponse(t, first, http.StatusServiceUnavailable, "modem", "unavailable")
	assertHealthResponse(t, second, http.StatusOK, "modem", "ok")
}

func TestDiagnosticReleasesGateAfterTimeout(t *testing.T) {
	var mutex sync.Mutex
	calls := 0
	server := newTestServer(t, 1, fakeGammu{
		devicePresent: func(ctx context.Context) error {
			mutex.Lock()
			calls++
			call := calls
			mutex.Unlock()
			if call == 1 {
				<-ctx.Done()
				return ctx.Err()
			}
			return nil
		},
	})

	first := executeRequest(httptest.NewRequest(http.MethodGet, "/health/modem", nil), server)
	second := executeRequest(httptest.NewRequest(http.MethodGet, "/health/modem", nil), server)

	assertHealthResponse(t, first, http.StatusServiceUnavailable, "modem", "timeout")
	assertHealthResponse(t, second, http.StatusOK, "modem", "ok")
}

func TestHealthResponsesAndLogsExcludeSensitiveData(t *testing.T) {
	const sensitive = "33612345678 secret-body IMSI-123 IMEI-456 OperatorName raw-gammu-output"
	var logs bytes.Buffer
	originalLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = originalLogger })

	server := newTestServer(t, 1, fakeGammu{
		devicePresent: func(context.Context) error { return errors.New(sensitive) },
		networkReady:  func(context.Context) (bool, error) { return false, errors.New(sensitive) },
	})

	for _, path := range []string{"/health/modem", "/health/network"} {
		response := executeRequest(httptest.NewRequest(http.MethodGet, path, nil), server)
		require.Equal(t, http.StatusServiceUnavailable, response.Code)
		require.NotContains(t, response.Body.String(), sensitive)
	}
	require.NotContains(t, logs.String(), sensitive)
}

func TestMalformedSMSDoesNotLogRequestBody(t *testing.T) {
	const sensitive = "33612345678 secret-body"
	var logs bytes.Buffer
	originalLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = originalLogger })
	server := newTestServer(t, 1, fakeGammu{})

	request := httptest.NewRequest(http.MethodPost, "/sendsms", io.NopCloser(strings.NewReader(`{"message":"`+sensitive)))
	response := executeRequest(request, server)

	require.Equal(t, http.StatusBadRequest, response.Code)
	require.NotContains(t, logs.String(), sensitive)
}

func TestPing(t *testing.T) {
	server := newTestServer(t, 1, fakeGammu{})
	response := executeRequest(httptest.NewRequest(http.MethodGet, "/ping", nil), server)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, ".", response.Body.String())
}
