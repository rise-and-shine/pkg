package mask_test

import (
	"testing"

	"github.com/rise-and-shine/pkg/mask"
	"github.com/stretchr/testify/assert"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

// Helper function to create an ordered map from key-value pairs.
func newOrderedMap(pairs ...any) *orderedmap.OrderedMap[string, any] {
	om := orderedmap.New[string, any]()
	for i := 0; i < len(pairs); i += 2 {
		key := pairs[i].(string)
		value := pairs[i+1]
		om.Set(key, value)
	}
	return om
}

// Helper function to compare ordered maps.
func assertOrderedMapEqual(t *testing.T, expected, actual *orderedmap.OrderedMap[string, any]) {
	t.Helper()

	if expected == nil && actual == nil {
		return
	}

	if expected == nil || actual == nil {
		t.Fatalf("one map is nil: expected=%v, actual=%v", expected, actual)
	}

	assert.Equal(t, expected.Len(), actual.Len(), "maps have different lengths")

	expectedPair := expected.Oldest()
	actualPair := actual.Oldest()

	for expectedPair != nil && actualPair != nil {
		assert.Equal(t, expectedPair.Key, actualPair.Key, "key mismatch")
		assert.Equal(t, expectedPair.Value, actualPair.Value, "value mismatch for key %s", expectedPair.Key)

		expectedPair = expectedPair.Next()
		actualPair = actualPair.Next()
	}
}

func TestStructToOrdMap_NilInput(t *testing.T) {
	result := mask.StructToOrdMap(nil)
	assert.Nil(t, result)
}

func TestStructToOrdMap_StringField(t *testing.T) {
	type Request struct {
		Username string
		Password string `mask:"true"`
	}

	req := Request{Username: "john", Password: "secret123"}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Username", "john",
		"Password", "***masked-string***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_StringFieldZeroValue(t *testing.T) {
	type Request struct {
		Username string
		Password string `mask:"true"`
	}

	req := Request{Username: "john", Password: ""}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Username", "john",
		"Password", "***masked-string***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_PointerToString(t *testing.T) {
	type Request struct {
		Username string
		Token    *string `mask:"true"`
	}

	token := "my-token"
	req := Request{Username: "john", Token: &token}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Username", "john",
		"Token", "***masked-string***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_PointerToStringNil(t *testing.T) {
	type Request struct {
		Username string
		Token    *string `mask:"true"`
	}

	req := Request{Username: "john", Token: nil}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Username", "john",
		"Token", nil,
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_IntField(t *testing.T) {
	type Config struct {
		Port   int
		Secret int `mask:"true"`
	}

	cfg := Config{Port: 8080, Secret: 12345}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"Port", 8080,
		"Secret", "***masked-int***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_IntFieldZeroValue(t *testing.T) {
	type Config struct {
		Port   int
		Secret int `mask:"true"`
	}

	cfg := Config{Port: 8080, Secret: 0}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"Port", 8080,
		"Secret", "***masked-int***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_UintField(t *testing.T) {
	type Config struct {
		ID     uint
		Secret uint `mask:"true"`
	}

	cfg := Config{ID: 1, Secret: 999}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"ID", uint(1),
		"Secret", "***masked-uint***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_FloatField(t *testing.T) {
	type Config struct {
		Rate   float64
		Secret float64 `mask:"true"`
	}

	cfg := Config{Rate: 1.5, Secret: 99.99}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"Rate", 1.5,
		"Secret", "***masked-float***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_BoolField(t *testing.T) {
	type Config struct {
		Enabled bool
		Secret  bool `mask:"true"`
	}

	cfg := Config{Enabled: true, Secret: true}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"Enabled", true,
		"Secret", "***masked-bool***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_BoolFieldZeroValue(t *testing.T) {
	type Config struct {
		Enabled bool
		Secret  bool `mask:"true"`
	}

	cfg := Config{Enabled: true, Secret: false}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"Enabled", true,
		"Secret", "***masked-bool***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_NestedStruct(t *testing.T) {
	type Credentials struct {
		Username string
		APIKey   string `mask:"true"`
	}
	type Request struct {
		Name  string
		Creds Credentials
	}

	req := Request{
		Name:  "test",
		Creds: Credentials{Username: "admin", APIKey: "sk-12345"},
	}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Name", "test",
		"Creds.Username", "admin",
		"Creds.APIKey", "***masked-string***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_NestedStructMasked(t *testing.T) {
	type Secret struct {
		Value string
	}
	type Request struct {
		Name string
		Data Secret `mask:"true"`
	}

	req := Request{Name: "test", Data: Secret{Value: "hidden"}}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Name", "test",
		"Data", "***masked-struct***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_PointerToStruct(t *testing.T) {
	type Credentials struct {
		Token string `mask:"true"`
	}
	type Request struct {
		Name  string
		Creds *Credentials
	}

	req := Request{Name: "test", Creds: &Credentials{Token: "abc"}}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Name", "test",
		"Creds.Token", "***masked-string***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_PointerToStructNil(t *testing.T) {
	type Credentials struct {
		Token string `mask:"true"`
	}
	type Request struct {
		Name  string
		Creds *Credentials
	}

	req := Request{Name: "test", Creds: nil}
	result := mask.StructToOrdMap(req)

	assert.Equal(t, 2, result.Len())

	val, ok := result.Get("Name")
	assert.True(t, ok)
	assert.Equal(t, "test", val)

	val, ok = result.Get("Creds")
	assert.True(t, ok)
	assert.Nil(t, val)
}

func TestStructToOrdMap_SliceField(t *testing.T) {
	type Request struct {
		Name string
		Tags []string
	}

	req := Request{Name: "test", Tags: []string{"a", "b"}}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Name", "test",
		"Tags", []string{"a", "b"},
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_SliceFieldMasked(t *testing.T) {
	type Request struct {
		Name string
		IDs  []int `mask:"true"`
	}

	req := Request{Name: "test", IDs: []int{1, 2, 3}}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Name", "test",
		"IDs", "***masked-slice***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_MapField(t *testing.T) {
	type Request struct {
		Name   string
		Config map[string]string
	}

	req := Request{Name: "test", Config: map[string]string{"key": "value"}}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Name", "test",
		"Config", map[string]string{"key": "value"},
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_MapFieldMasked(t *testing.T) {
	type Request struct {
		Name   string
		Config map[string]string `mask:"true"`
	}

	req := Request{Name: "test", Config: map[string]string{"key": "value"}}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Name", "test",
		"Config", "***masked-map***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_PointerInput(t *testing.T) {
	type Request struct {
		Username string
		Password string `mask:"true"`
	}

	req := &Request{Username: "john", Password: "secret"}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Username", "john",
		"Password", "***masked-string***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_TagCaseInsensitive(t *testing.T) {
	type Request struct {
		A string `mask:"TRUE"`
		B string `mask:"True"`
		C string `mask:"true"`
	}

	req := Request{A: "aaa", B: "bbb", C: "ccc"}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"A", "***masked-string***",
		"B", "***masked-string***",
		"C", "***masked-string***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_NoMaskTag(t *testing.T) {
	type Request struct {
		Public  string
		Private string `mask:"false"`
	}

	req := Request{Public: "pub", Private: "priv"}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Public", "pub",
		"Private", "priv",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_UnexportedFields(t *testing.T) {
	type Request struct {
		Public  string
		private string
	}

	req := Request{Public: "pub"}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Public", "pub",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_DeeplyNested(t *testing.T) {
	type Level3 struct {
		Secret string `mask:"true"`
	}
	type Level2 struct {
		Data Level3
	}
	type Level1 struct {
		Info Level2
	}

	req := Level1{Info: Level2{Data: Level3{Secret: "deep"}}}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Info.Data.Secret", "***masked-string***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_MixedTypes(t *testing.T) {
	type Request struct {
		Name     string
		Password string  `mask:"true"`
		Age      int     `mask:"true"`
		Rate     float64 `mask:"true"`
		Active   bool    `mask:"true"`
	}

	req := Request{
		Name:     "john",
		Password: "secret",
		Age:      30,
		Rate:     1.5,
		Active:   true,
	}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Name", "john",
		"Password", "***masked-string***",
		"Age", "***masked-int***",
		"Rate", "***masked-float***",
		"Active", "***masked-bool***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_PointerToInt(t *testing.T) {
	type Config struct {
		ID     int
		Secret *int `mask:"true"`
	}

	secret := 42
	cfg := Config{ID: 1, Secret: &secret}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"ID", 1,
		"Secret", "***masked-int***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_PointerToIntNil(t *testing.T) {
	type Config struct {
		ID     int
		Secret *int `mask:"true"`
	}

	cfg := Config{ID: 1, Secret: nil}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"ID", 1,
		"Secret", nil,
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_EmptyStruct(t *testing.T) {
	type Empty struct{}

	result := mask.StructToOrdMap(Empty{})

	assert.Equal(t, 0, result.Len())
}

func TestStructToOrdMap_NestedPointerChain(t *testing.T) {
	type Inner struct {
		Value string `mask:"true"`
	}
	type Middle struct {
		Inner *Inner
	}
	type Outer struct {
		Middle *Middle
	}

	req := Outer{Middle: &Middle{Inner: &Inner{Value: "secret"}}}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Middle.Inner.Value", "***masked-string***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_MultipleNestedStructs(t *testing.T) {
	type Auth struct {
		Token string `mask:"true"`
	}
	type Meta struct {
		TraceID string
	}
	type Request struct {
		Auth Auth
		Meta Meta
	}

	req := Request{
		Auth: Auth{Token: "abc123"},
		Meta: Meta{TraceID: "trace-1"},
	}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Auth.Token", "***masked-string***",
		"Meta.TraceID", "trace-1",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_FieldOrdering(t *testing.T) {
	type Request struct {
		Z string
		A string
		M string
	}

	req := Request{Z: "z", A: "a", M: "m"}
	result := mask.StructToOrdMap(req)

	// Should preserve struct field order, not alphabetical
	expected := newOrderedMap(
		"Z", "z",
		"A", "a",
		"M", "m",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_Int8Field(t *testing.T) {
	type Config struct {
		Secret int8 `mask:"true"`
	}

	cfg := Config{Secret: 42}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"Secret", "***masked-int***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_Int16Field(t *testing.T) {
	type Config struct {
		Secret int16 `mask:"true"`
	}

	cfg := Config{Secret: 42}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"Secret", "***masked-int***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_Int32Field(t *testing.T) {
	type Config struct {
		Secret int32 `mask:"true"`
	}

	cfg := Config{Secret: 42}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"Secret", "***masked-int***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_Int64Field(t *testing.T) {
	type Config struct {
		Secret int64 `mask:"true"`
	}

	cfg := Config{Secret: 42}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"Secret", "***masked-int***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_Uint8Field(t *testing.T) {
	type Config struct {
		Secret uint8 `mask:"true"`
	}

	cfg := Config{Secret: 42}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"Secret", "***masked-uint***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_Uint16Field(t *testing.T) {
	type Config struct {
		Secret uint16 `mask:"true"`
	}

	cfg := Config{Secret: 42}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"Secret", "***masked-uint***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_Uint32Field(t *testing.T) {
	type Config struct {
		Secret uint32 `mask:"true"`
	}

	cfg := Config{Secret: 42}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"Secret", "***masked-uint***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_Uint64Field(t *testing.T) {
	type Config struct {
		Secret uint64 `mask:"true"`
	}

	cfg := Config{Secret: 42}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"Secret", "***masked-uint***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_Float32Field(t *testing.T) {
	type Config struct {
		Secret float32 `mask:"true"`
	}

	cfg := Config{Secret: 3.14}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"Secret", "***masked-float***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_ArrayField(t *testing.T) {
	type Config struct {
		Codes [3]int `mask:"true"`
	}

	cfg := Config{Codes: [3]int{1, 2, 3}}
	result := mask.StructToOrdMap(cfg)

	expected := newOrderedMap(
		"Codes", "***masked-slice***",
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_NilSliceField(t *testing.T) {
	type Request struct {
		Tags []string
	}

	req := Request{Tags: nil}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Tags", ([]string)(nil),
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_NilMapField(t *testing.T) {
	type Request struct {
		Config map[string]string
	}

	req := Request{Config: nil}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Config", (map[string]string)(nil),
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_NilSliceFieldMasked(t *testing.T) {
	type Request struct {
		IDs []int `mask:"true"`
	}

	req := Request{IDs: nil}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"IDs", nil,
	)
	assertOrderedMapEqual(t, expected, result)
}

func TestStructToOrdMap_NilMapFieldMasked(t *testing.T) {
	type Request struct {
		Config map[string]string `mask:"true"`
	}

	req := Request{Config: nil}
	result := mask.StructToOrdMap(req)

	expected := newOrderedMap(
		"Config", nil,
	)
	assertOrderedMapEqual(t, expected, result)
}
