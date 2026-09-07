package main

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type recordingRunner struct {
	name   string
	args   []string
	output []byte
	err    error
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.name = name
	r.args = append([]string(nil), args...)
	return r.output, r.err
}

func TestGammuSendCommandIsInjectable(t *testing.T) {
	runner := &recordingRunner{}
	client := NewGammuClient("/etc/gammu-test.conf", runner)

	err := client.SendSMS(
		context.Background(),
		SMS{PhoneNumber: "+15555550100", Message: "test message"},
	)

	require.NoError(t, err)
	require.Equal(t, "gammu", runner.name)
	require.Equal(t, []string{
		"-c",
		"/etc/gammu-test.conf",
		"--sendsms",
		"TEXT",
		"+15555550100",
		"-text",
		"test message",
		"-autolen",
		"12",
	}, runner.args)
}

func TestGammuCommandFailureDoesNotExposeOutput(t *testing.T) {
	runner := &recordingRunner{
		output: []byte("sensitive modem output"),
		err:    errors.New("command failed"),
	}
	client := NewGammuClient("/etc/gammu-test.conf", runner)

	err := client.SendSMS(
		context.Background(),
		SMS{PhoneNumber: "+15555550100", Message: "test message"},
	)

	require.EqualError(t, err, "gammu sendsms failed")
	require.NotContains(t, err.Error(), string(runner.output))
}

func TestNetworkRegistered(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		registered bool
	}{
		{name: "home", output: "State : registered home network", registered: true},
		{name: "home alternative", output: "Network state : home network", registered: true},
		{name: "roaming", output: "State : roaming network", registered: true},
		{name: "searching", output: "State : searching", registered: false},
		{name: "denied", output: "State : registration denied", registered: false},
		{name: "missing state", output: "Network : 001/01", registered: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.registered, networkRegistered(test.output))
		})
	}
}
