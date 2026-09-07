package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

var ErrNetworkNotReady = errors.New("mobile network is not registered")

type Gammu interface {
	SendSMS(context.Context, SMS) error
	DevicePresent(context.Context) error
	NetworkReady(context.Context) error
}

type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type ExecCommandRunner struct{}

func (ExecCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

type GammuClient struct {
	configPath string
	runner     CommandRunner
}

func NewGammuClient(configPath string, runner CommandRunner) *GammuClient {
	return &GammuClient{configPath: configPath, runner: runner}
}

func (g *GammuClient) SendSMS(ctx context.Context, sms SMS) error {
	_, err := g.run(
		ctx,
		"sendsms",
		"--sendsms",
		"TEXT",
		sms.PhoneNumber,
		"-text",
		sms.Message,
		"-autolen",
		strconv.Itoa(len(sms.Message)),
	)
	return err
}

func (g *GammuClient) DevicePresent(ctx context.Context) error {
	_, err := g.run(ctx, "identify", "identify")
	return err
}

func (g *GammuClient) NetworkReady(ctx context.Context) error {
	output, err := g.run(ctx, "getnetworkinfo", "getnetworkinfo")
	if err != nil {
		return err
	}
	if !networkRegistered(string(output)) {
		return ErrNetworkNotReady
	}
	return nil
}

func (g *GammuClient) run(ctx context.Context, operation string, args ...string) ([]byte, error) {
	output, err := g.runner.Run(ctx, "gammu", append([]string{"-c", g.configPath}, args...)...)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err == nil {
		return output, nil
	}

	exitCode := -1
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	}
	return nil, &GammuCommandError{Operation: operation, ExitCode: exitCode, Err: err}
}

type GammuCommandError struct {
	Operation string
	ExitCode  int
	Err       error
}

func (e *GammuCommandError) Error() string {
	if e.ExitCode >= 0 {
		return fmt.Sprintf("gammu %s failed with exit code %d", e.Operation, e.ExitCode)
	}
	return fmt.Sprintf("gammu %s failed", e.Operation)
}

func (e *GammuCommandError) Unwrap() error {
	return e.Err
}

func networkRegistered(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(strings.ToLower(strings.TrimSpace(line)), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if key != "state" && key != "network state" {
			continue
		}
		state := strings.TrimSpace(parts[1])
		return strings.Contains(state, "registered home") ||
			strings.Contains(state, "registered (home") ||
			state == "home network" ||
			strings.Contains(state, "roaming network") ||
			strings.Contains(state, "registered (roaming")
	}
	return false
}
