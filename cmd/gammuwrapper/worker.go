package main

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

// Worker responsible for queue serving.
type Worker struct {
	Queue       *BQueue
	Gammu       GammuOperations
	Gate        *ModemGate
	SendTimeout time.Duration
}

// NewWorker initializes a new Worker.
func NewWorker(queue *BQueue, gammu GammuOperations, gate *ModemGate, sendTimeout time.Duration) *Worker {
	return &Worker{
		Queue:       queue,
		Gammu:       gammu,
		Gate:        gate,
		SendTimeout: sendTimeout,
	}
}

// DoWork processes jobs from the queue (jobs channel).
func (w *Worker) WaitForSMS() bool {
	for {
		select {
		// if context was canceled.
		case <-w.Queue.ctx.Done():
			log.Printf("Work done in queue %s: %s!", w.Queue.name, w.Queue.ctx.Err())
			return true
		// if job received.
		case job := <-w.Queue.channel:
			w.send(job)
		}
	}
}

func (w *Worker) send(sms SMS) {
	if err := w.Gate.Acquire(w.Queue.ctx); err != nil {
		return
	}
	defer w.Gate.Release()

	ctx, cancel := context.WithTimeout(w.Queue.ctx, w.SendTimeout)
	defer cancel()

	if err := w.Gammu.SendSMS(ctx, sms); err != nil {
		log.Error().Msg("SMS send failed")
	}
}
