package main

import "context"

type ModemGate struct {
	token chan struct{}
}

func NewModemGate() *ModemGate {
	gate := &ModemGate{token: make(chan struct{}, 1)}
	gate.token <- struct{}{}
	return gate
}

func (g *ModemGate) Acquire(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-g.token:
		return nil
	}
}

func (g *ModemGate) Release() {
	g.token <- struct{}{}
}
