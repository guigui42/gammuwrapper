package main

import (
	"context"
	"errors"
)

var (
	ErrQueueEmpty = errors.New("queue empty")
	ErrQueueFull  = errors.New("queue full")
)

type BQueue struct {
	name    string
	channel chan SMS
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewQueue(capacity int) *BQueue {
	ctx, cancel := context.WithCancel(context.Background())
	return &BQueue{
		channel: make(chan SMS, capacity),
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (q *BQueue) Enqueue(sms SMS) error {
	select {
	case q.channel <- sms:
		return nil
	default:
		return ErrQueueFull
	}
}
