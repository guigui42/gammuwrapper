package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeGammu struct {
	sendSMS       func(context.Context, SMS) error
	devicePresent func(context.Context) error
	networkReady  func(context.Context) error
}

func (f *fakeGammu) SendSMS(ctx context.Context, sms SMS) error {
	if f.sendSMS == nil {
		return nil
	}
	return f.sendSMS(ctx, sms)
}

func (f *fakeGammu) DevicePresent(ctx context.Context) error {
	if f.devicePresent == nil {
		return nil
	}
	return f.devicePresent(ctx)
}

func (f *fakeGammu) NetworkReady(ctx context.Context) error {
	if f.networkReady == nil {
		return nil
	}
	return f.networkReady(ctx)
}

func testServer(gammu Gammu, timeout time.Duration) *Server {
	server := newServer(gammu, timeout)
	server.MountHandlers()
	return server
}

func executeRequest(req *http.Request, server *Server) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, req)
	return response
}

func smsRequest(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/sendsms", bytes.NewBufferString(body))
}

func TestSendSMSSuccess(t *testing.T) {
	var received SMS
	server := testServer(&fakeGammu{
		sendSMS: func(_ context.Context, sms SMS) error {
			received = sms
			return nil
		},
	}, time.Second)

	response := executeRequest(
		smsRequest(`{"phone_number":"+15555550100","message":"test message"}`),
		server,
	)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "SMS sent", response.Body.String())
	require.Equal(t, SMS{PhoneNumber: "+15555550100", Message: "test message"}, received)
}

func TestSendSMSValidation(t *testing.T) {
	var calls atomic.Int32
	server := testServer(&fakeGammu{
		sendSMS: func(context.Context, SMS) error {
			calls.Add(1)
			return nil
		},
	}, time.Second)

	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{`},
		{name: "missing phone number", body: `{"message":"test"}`},
		{name: "missing message", body: `{"phone_number":"+15555550100"}`},
		{name: "multiple objects", body: `{} {}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := executeRequest(smsRequest(test.body), server)
			require.Equal(t, http.StatusBadRequest, response.Code)
		})
	}
	require.Zero(t, calls.Load())
}

func TestSendSMSTimeout(t *testing.T) {
	server := testServer(&fakeGammu{
		sendSMS: func(ctx context.Context, _ SMS) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}, 20*time.Millisecond)

	response := executeRequest(
		smsRequest(`{"phone_number":"+15555550100","message":"test"}`),
		server,
	)

	require.Equal(t, http.StatusGatewayTimeout, response.Code)
	require.Contains(t, response.Body.String(), "timed out")
}

func TestSendSMSNonZeroGammuExit(t *testing.T) {
	server := testServer(&fakeGammu{
		sendSMS: func(context.Context, SMS) error {
			return &GammuCommandError{
				Operation: "sendsms",
				ExitCode:  127,
				Err:       errors.New("exit status 127"),
			}
		},
	}, time.Second)

	response := executeRequest(
		smsRequest(`{"phone_number":"+15555550100","message":"test"}`),
		server,
	)

	require.Equal(t, http.StatusBadGateway, response.Code)
	require.Equal(t, "Gammu failed to send SMS\n", response.Body.String())
}

func TestConcurrentSendRejectedWhileModemBusy(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := testServer(&fakeGammu{
		sendSMS: func(context.Context, SMS) error {
			close(started)
			<-release
			return nil
		},
	}, time.Second)

	firstDone := make(chan *httptest.ResponseRecorder)
	go func() {
		firstDone <- executeRequest(
			smsRequest(`{"phone_number":"+15555550100","message":"first"}`),
			server,
		)
	}()
	<-started

	second := executeRequest(
		smsRequest(`{"phone_number":"+15555550101","message":"second"}`),
		server,
	)
	require.Equal(t, http.StatusTooManyRequests, second.Code)

	close(release)
	first := <-firstDone
	require.Equal(t, http.StatusOK, first.Code)
}

func TestHealthEndpoints(t *testing.T) {
	server := testServer(&fakeGammu{
		devicePresent: func(context.Context) error {
			return nil
		},
		networkReady: func(context.Context) error {
			return ErrNetworkNotReady
		},
	}, time.Second)

	httpHealth := executeRequest(httptest.NewRequest(http.MethodGet, "/health", nil), server)
	require.Equal(t, http.StatusOK, httpHealth.Code)
	require.JSONEq(t, `{"component":"http","status":"ok"}`, httpHealth.Body.String())

	modemHealth := executeRequest(httptest.NewRequest(http.MethodGet, "/health/modem", nil), server)
	require.Equal(t, http.StatusOK, modemHealth.Code)
	require.JSONEq(t, `{"component":"modem","status":"ok"}`, modemHealth.Body.String())

	networkHealth := executeRequest(httptest.NewRequest(http.MethodGet, "/health/network", nil), server)
	require.Equal(t, http.StatusServiceUnavailable, networkHealth.Code)
	require.JSONEq(
		t,
		`{"component":"network","status":"unavailable","message":"network check failed"}`,
		networkHealth.Body.String(),
	)
}

func TestPing(t *testing.T) {
	server := testServer(&fakeGammu{}, time.Second)
	response := executeRequest(httptest.NewRequest(http.MethodGet, "/ping", nil), server)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, ".", response.Body.String())
}
