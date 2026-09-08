package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type runnerCall struct {
	name string
	args []string
}

type fakeRunner struct {
	output []byte
	err    error
	calls  []runnerCall
	run    func(context.Context, string, ...string) ([]byte, error)
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, runnerCall{name: name, args: append([]string(nil), args...)})
	if r.run != nil {
		return r.run(ctx, name, args...)
	}
	return r.output, r.err
}

func TestGammuCommands(t *testing.T) {
	tests := []struct {
		name         string
		call         func(*GammuClient) error
		expectedArgs []string
	}{
		{
			name: "send SMS",
			call: func(client *GammuClient) error {
				return client.SendSMS(context.Background(), SMS{PhoneNumber: "33600000000", Message: "hello"})
			},
			expectedArgs: []string{"-c", "/config", "--sendsms", "TEXT", "33600000000", "-text", "hello", "-autolen", "5"},
		},
		{
			name:         "identify modem",
			call:         func(client *GammuClient) error { return client.DevicePresent(context.Background()) },
			expectedArgs: []string{"-c", "/config", "identify"},
		},
		{
			name: "network info",
			call: func(client *GammuClient) error {
				_, err := client.NetworkReady(context.Background())
				return err
			},
			expectedArgs: []string{"-c", "/config", "networkinfo"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeRunner{output: []byte("Network state : home network")}
			client := NewGammuClient("/config", runner)
			require.NoError(t, test.call(client))
			require.Equal(t, []runnerCall{{name: "gammu", args: test.expectedArgs}}, runner.calls)
		})
	}
}

func TestNetworkReadyParsing(t *testing.T) {
	tests := []struct {
		name   string
		output string
		ready  bool
	}{
		{name: "home network", output: "Network state : home network\nNetwork : 20801", ready: true},
		{name: "roaming network", output: "State: roaming network\nName: Private Operator", ready: true},
		{name: "requesting network", output: "Network state : requesting network", ready: false},
		{name: "searching", output: "Network state : searching", ready: false},
		{name: "denied", output: "Network state : denied", ready: false},
		{name: "detached", output: "Network state : detached", ready: false},
		{name: "unknown", output: "Network state : unknown", ready: false},
		{name: "missing state", output: "Network : 20801\nName : Private Operator", ready: false},
		{name: "malformed state", output: "Network state home network", ready: false},
		{name: "unrelated state", output: "Packet state : home network", ready: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.ready, parseNetworkReady([]byte(test.output)))
		})
	}
}

func TestGammuClientReturnsCommandFailuresAndTimeouts(t *testing.T) {
	commandFailure := errors.New("command failed with 33612345678 secret-body IMSI-123 IMEI-456 OperatorName raw-output")
	client := NewGammuClient("/config", &fakeRunner{err: commandFailure})

	sendErr := client.SendSMS(context.Background(), SMS{})
	require.ErrorIs(t, sendErr, errGammuCommandFailed)
	require.NotContains(t, sendErr.Error(), commandFailure.Error())

	modemErr := client.DevicePresent(context.Background())
	require.ErrorIs(t, modemErr, errGammuCommandFailed)
	require.NotContains(t, modemErr.Error(), commandFailure.Error())

	ready, err := client.NetworkReady(context.Background())
	require.False(t, ready)
	require.ErrorIs(t, err, errGammuCommandFailed)
	require.NotContains(t, err.Error(), commandFailure.Error())

	tests := []struct {
		name string
		call func(*GammuClient, context.Context) error
	}{
		{
			name: "send SMS",
			call: func(client *GammuClient, ctx context.Context) error {
				return client.SendSMS(ctx, SMS{})
			},
		},
		{
			name: "identify modem",
			call: func(client *GammuClient, ctx context.Context) error {
				return client.DevicePresent(ctx)
			},
		},
		{
			name: "network info",
			call: func(client *GammuClient, ctx context.Context) error {
				_, err := client.NetworkReady(ctx)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name+" timeout", func(t *testing.T) {
			timeoutClient := NewGammuClient("/config", &fakeRunner{
				run: func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
					<-ctx.Done()
					return nil, ctx.Err()
				},
			})
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
			defer cancel()
			require.ErrorIs(t, test.call(timeoutClient, ctx), context.DeadlineExceeded)
		})
	}
}
