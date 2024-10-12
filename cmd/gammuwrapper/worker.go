package main

import (
	"github.com/rs/zerolog/log"
)

// Worker responsible for queue serving.
type Worker struct {
	Queue *BQueue
}

// NewWorker initializes a new Worker.
func NewWorker(queue *BQueue) *Worker {
	return &Worker{
		Queue: queue,
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
			_, err := sendSMS(w.Queue.ctx, job)
			if err != nil {
				log.Error().Err(err).Msgf("Error sending SMS: %v", err)
				continue
			}
		}
	}
}
