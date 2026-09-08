package main

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

var errGammuCommandFailed = errors.New("gammu command failed")

// CommandRunner executes an external command with context cancellation.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ExecCommandRunner runs commands through os/exec.
type ExecCommandRunner struct{}

func (ExecCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// GammuOperations contains the Gammu capabilities used by the server.
type GammuOperations interface {
	SendSMS(ctx context.Context, sms SMS) error
	DevicePresent(ctx context.Context) error
	NetworkReady(ctx context.Context) (bool, error)
}

// GammuClient invokes Gammu through an injectable command runner.
type GammuClient struct {
	configPath string
	runner     CommandRunner
}

// NewGammuClient creates a client for a Gammu configuration file.
func NewGammuClient(configPath string, runner CommandRunner) *GammuClient {
	return &GammuClient{configPath: configPath, runner: runner}
}

// SendSMS submits one SMS to Gammu.
func (g *GammuClient) SendSMS(ctx context.Context, sms SMS) error {
	_, err := g.runner.Run(
		ctx,
		"gammu",
		"-c",
		g.configPath,
		"--sendsms",
		"TEXT",
		sms.PhoneNumber,
		"-text",
		sms.Message,
		"-autolen",
		strconv.Itoa(len(sms.Message)),
	)
	return safeCommandError(ctx, err)
}

// DevicePresent checks whether Gammu can identify the configured modem.
func (g *GammuClient) DevicePresent(ctx context.Context) error {
	_, err := g.runner.Run(ctx, "gammu", "-c", g.configPath, "identify")
	return safeCommandError(ctx, err)
}

// NetworkReady reports whether Gammu shows home or roaming registration.
func (g *GammuClient) NetworkReady(ctx context.Context) (bool, error) {
	output, err := g.runner.Run(ctx, "gammu", "-c", g.configPath, "networkinfo")
	if err != nil {
		return false, safeCommandError(ctx, err)
	}
	return parseNetworkReady(output), nil
}

func safeCommandError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	return errGammuCommandFailed
}

func parseNetworkReady(output []byte) bool {
	for _, line := range strings.Split(string(output), "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}

		switch strings.ToLower(strings.TrimSpace(key)) {
		case "state", "network state":
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "home network", "roaming network":
				return true
			default:
				return false
			}
		}
	}
	return false
}
