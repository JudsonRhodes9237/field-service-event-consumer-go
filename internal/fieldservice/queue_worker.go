package fieldservice

import (
	"context"

	"github.com/example/field-service-event-consumer-go/internal/infrai"
)

type QueueWorker struct {
	client    *infrai.Client
	queueName string
	processor *Processor
}

func NewQueueWorker(client *infrai.Client, queueName string, processor *Processor) *QueueWorker {
	return &QueueWorker{client: client, queueName: queueName, processor: processor}
}

func (w *QueueWorker) RunOnce(ctx context.Context) error {
	messages, err := w.client.QueueConsume(ctx, w.queueName, infrai.QueueConsumeRequest{
		MaxMessages:       10,
		VisibilityTimeout: 30,
	})
	if err != nil {
		return err
	}

	for _, msg := range messages {
		evt, err := DecodeEvent(msg.Payload)
		if err != nil {
			continue
		}
		w.processor.Process(evt)
		if err := w.client.QueueAck(ctx, w.queueName, msg.MessageID); err != nil {
			return err
		}
	}
	return nil
}
