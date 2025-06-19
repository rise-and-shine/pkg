package forward

import (
	"reflect"

	"slices"

	"github.com/code19m/errx"
)

const (
	codeInvalidContentType = "INVALID_CONTENT_TYPE"
	codeInvalidJSONBody    = "INVALID_JSON_BODY"
	codeInvalidQueryParams = "INVALID_QUERY_PARAMS"
	codeInvalidPathParams  = "INVALID_PATH_PARAMS"
)

func newRequest[T_Req any]() (T_Req, error) {
	var req T_Req

	reqType := reflect.TypeOf((*T_Req)(nil)).Elem()
	if reqType.Kind() != reflect.Ptr || reqType.Elem().Kind() != reflect.Struct {
		return req, errx.New("T_Req must be a pointer to a use case input struct")
	}

	reqVal := reflect.New(reqType.Elem()).Interface().(T_Req) //nolint:errcheck // safe type assertion
	return reqVal, nil
}

func isJSONMethod(method string) bool {
	return slices.Contains([]string{"POST", "PUT", "PATCH"}, method)
}
