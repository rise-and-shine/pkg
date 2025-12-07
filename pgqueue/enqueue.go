package pgqueue

import (
	"context"

	"github.com/code19m/errx"
	"github.com/uptrace/bun"
)

// EnqueueBatchTx adds multiple messages to the queue within a transaction.
// All messages in the batch will be enqueued to the same queue.
// Returns a list of message IDs for the enqueued messages.
func (q *queue) EnqueueBatchTx(
	ctx context.Context,
	tx *bun.Tx,
	queueName string,
	messages []SingleMessage,
) ([]int64, error) {
	// Validate queue name
	if queueName == "" {
		return nil, errx.New("[pgqueue]: queue name is required")
	}

	if len(messages) == 0 {
		return []int64{}, nil
	}

	messageIDs := make([]int64, 0, len(messages))

	for _, msg := range messages {
		id, err := q.enqueueSingle(ctx, tx, queueName, msg)
		if err != nil {
			return nil, errx.Wrap(err)
		}
		messageIDs = append(messageIDs, id)
	}

	return messageIDs, nil
}

// enqueueSingle adds a single message to the queue within a transaction.
// This is an internal helper used by EnqueueBatch.
func (q *queue) enqueueSingle(
	ctx context.Context,
	db bun.IDB,
	queueName string,
	singleMsg SingleMessage,
) (int64, error) {
	// Validate
	err := validateSingleMsg(singleMsg)
	if err != nil {
		return 0, errx.Wrap(err)
	}

	// Normalize payload
	payload := singleMsg.Payload
	if payload == nil {
		payload = map[string]any{}
	}

	// Create the message
	msg := &Message{
		QueueName:      queueName,
		MessageGroupID: singleMsg.MessageGroupID,
		Payload:        payload,
		ScheduledAt:    singleMsg.ScheduledAt,
		VisibleAt:      singleMsg.ScheduledAt, // Initially visible at scheduled time
		Priority:       singleMsg.Priority,
		MaxAttempts:    singleMsg.MaxAttempts,
		ExpiresAt:      singleMsg.ExpiresAt,
		IdempotencyKey: singleMsg.IdempotencyKey,
		Attempts:       0,
	}

	// Insert the message
	err = q.insertMessage(ctx, db, msg)
	if err != nil {
		return 0, errx.Wrap(err)
	}

	return msg.ID, nil
}
