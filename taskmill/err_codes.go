package taskmill

// Error codes for taskmill operations.
const (
	// CodeScheduleNotFound is returned when a schedule is not found.
	CodeScheduleNotFound = "SCHEDULE_NOT_FOUND"

	// CodeTaskNotRegistered is returned when trying to process a task that hasn't been registered.
	CodeTaskNotRegistered = "TASK_NOT_REGISTERED"

	// CodeInvalidPayload is returned when the message payload is invalid or malformed.
	CodeInvalidPayload = "INVALID_PAYLOAD"
)
