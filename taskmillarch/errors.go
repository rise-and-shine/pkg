package taskmill

import "github.com/code19m/errx"

var (
	// ErrTaskNotRegistered is returned when trying to execute an unregistered task.
	ErrTaskNotRegistered = errx.New("[taskmill]: task not registered")

	// ErrTaskAlreadyRegistered is returned when trying to register a task that already exists.
	ErrTaskAlreadyRegistered = errx.New("[taskmill]: task already registered")

	// ErrInvalidCronPattern is returned when a cron pattern is invalid.
	ErrInvalidCronPattern = errx.New("[taskmill.scheduler]: invalid cron pattern")

	// ErrPayloadSerializationFailed is returned when payload serialization fails.
	ErrPayloadSerializationFailed = errx.New("[taskmill]: payload serialization failed")

	// ErrPayloadDeserializationFailed is returned when payload deserialization fails.
	ErrPayloadDeserializationFailed = errx.New("[taskmill]: payload deserialization failed")

	// ErrWorkerStopped is returned when trying to operate on a stopped worker.
	ErrWorkerStopped = errx.New("[taskmill.worker]: worker stopped")

	// ErrSchedulerStopped is returned when trying to operate on a stopped scheduler.
	ErrSchedulerStopped = errx.New("[taskmill.scheduler]: scheduler stopped")

	// ErrInvalidSchedule is returned when a schedule is invalid.
	ErrInvalidSchedule = errx.New("[taskmill.scheduler]: invalid schedule")

	// ErrPayloadTypeMismatch is returned when payload type doesn't match task handler expectation.
	ErrPayloadTypeMismatch = errx.New("[taskmill]: payload type mismatch")
)
