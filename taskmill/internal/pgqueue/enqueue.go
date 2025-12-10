package pgqueue

import (
	"context"

	"github.com/code19m/errx"
	"github.com/uptrace/bun"
)

const (
	CodeDuplicateMessage = "DUPLICATE_MESSAGE"
)

// EnqueueBatch adds multiple messages to the queue.
// All messages in the batch will be enqueued to the same queue.
// Returns a list of message IDs for the enqueued messages.
// If any message has a duplicate idempotency key, the entire batch fails with CodeDuplicateMessage.
func (q *queue) EnqueueBatch(
	ctx context.Context,
	db bun.IDB,
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

	// Validate all messages first
	for _, msg := range messages {
		err := validateSingleMsg(msg)
		if err != nil {
			return nil, errx.Wrap(err, errx.WithDetails(errx.D{"message": msg}))
		}
	}

	// Convert SingleMessage to Message structs
	dbMessages := make([]Message, 0, len(messages))
	for _, singleMsg := range messages {
		// Normalize payload
		payload := singleMsg.Payload
		if payload == nil {
			payload = map[string]any{}
		}

		dbMessages = append(dbMessages, Message{
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
		})
	}

	// Insert all messages in a single batch
	ids, err := q.insertMessages(ctx, db, dbMessages)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return ids, nil
}
