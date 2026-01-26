package worker

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/ucdef"
)

// ForwardToAsyncTask registers a typed AsyncTask[P] with the worker.
// P must be a pointer to a struct.
// This is a standalone function because Go doesn't support generic methods.
func ForwardToAsyncTask[P any](w Worker, task ucdef.AsyncTask[P]) {
	w.RegisterAsyncTask(&asyncTaskAdapter[P]{uc: task})
}

type asyncTaskAdapter[P any] struct {
	uc ucdef.AsyncTask[P]
}

func (a *asyncTaskAdapter[P]) OperationID() string {
	return a.uc.OperationID()
}

func (a *asyncTaskAdapter[P]) Execute(ctx context.Context, rawPayload any) error {
	// Create typed payload instance
	payload, err := newPayload[P]()
	if err != nil {
		return errx.Wrap(err, errx.WithCode(CodeInvalidPayload))
	}

	// Convert raw payload (map[string]any from JSONB) to typed struct
	if err = unmarshalPayload(rawPayload, payload); err != nil {
		return errx.Wrap(err, errx.WithCode(CodeInvalidPayload))
	}

	return a.uc.Execute(ctx, payload)
}

func newPayload[P any]() (P, error) {
	var payload P
	payloadType := reflect.TypeOf((*P)(nil)).Elem()
	if payloadType.Kind() != reflect.Pointer || payloadType.Elem().Kind() != reflect.Struct {
		return payload, errx.New("payload type P must be a pointer to a struct")
	}

	payload, ok := reflect.New(payloadType.Elem()).Interface().(P)
	if !ok {
		return payload, errx.New("failed to create payload instance")
	}

	return payload, nil
}

func unmarshalPayload(raw any, target any) error {
	if raw == nil {
		return errx.New("received empty task payload", errx.WithCode(CodeInvalidPayload))
	}
	// Convert map[string]any (from JSONB) to typed struct via JSON round-trip
	jsonBytes, err := json.Marshal(raw)
	if err != nil {
		return errx.Wrap(err)
	}
	return json.Unmarshal(jsonBytes, target)
}
