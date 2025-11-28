package typed

import (
	"reflect"
	"testing"
)

func TestStructToSignature(t *testing.T) {
	type TestStruct struct {
		Input  string `dsgo:"input,desc=Input text"`
		Output string `dsgo:"output,desc=Output text"`
	}

	sig, err := StructToSignature(reflect.TypeOf(TestStruct{}), "test signature")
	if err != nil {
		t.Fatalf("StructToSignature() error = %v", err)
	}

	if sig.Description != "test signature" {
		t.Errorf("Description = %q, want %q", sig.Description, "test signature")
	}

	if len(sig.InputFields) != 1 {
		t.Errorf("InputFields count = %d, want 1", len(sig.InputFields))
	}

	if len(sig.OutputFields) != 1 {
		t.Errorf("OutputFields count = %d, want 1", len(sig.OutputFields))
	}

	if sig.InputFields[0].Name != "Input" {
		t.Errorf("InputField name = %q, want %q", sig.InputFields[0].Name, "Input")
	}

	if sig.OutputFields[0].Name != "Output" {
		t.Errorf("OutputField name = %q, want %q", sig.OutputFields[0].Name, "Output")
	}
}

func TestStructToMap(t *testing.T) {
	type TestStruct struct {
		Input  string `dsgo:"input,desc=Input text"`
		Output string `dsgo:"output,desc=Output text"`
		NoTag  string // Should be ignored
	}

	s := TestStruct{
		Input:  "test input",
		Output: "test output",
		NoTag:  "ignored",
	}

	m, err := StructToMap(s)
	if err != nil {
		t.Fatalf("StructToMap() error = %v", err)
	}

	if len(m) != 2 {
		t.Errorf("map length = %d, want 2", len(m))
	}

	if m["Input"] != "test input" {
		t.Errorf("Input = %q, want %q", m["Input"], "test input")
	}

	if m["Output"] != "test output" {
		t.Errorf("Output = %q, want %q", m["Output"], "test output")
	}

	if _, exists := m["NoTag"]; exists {
		t.Error("NoTag should not be in map")
	}
}

func TestStructToMap_Pointer(t *testing.T) {
	type TestStruct struct {
		Field string `dsgo:"input,desc=Test"`
	}

	s := &TestStruct{Field: "value"}

	m, err := StructToMap(s)
	if err != nil {
		t.Fatalf("StructToMap() error = %v", err)
	}

	if m["Field"] != "value" {
		t.Errorf("Field = %q, want %q", m["Field"], "value")
	}
}

func TestStructToMap_NotStruct(t *testing.T) {
	_, err := StructToMap("not a struct")
	if err == nil {
		t.Error("StructToMap() should return error for non-struct")
	}
}

func TestMapToStruct(t *testing.T) {
	type TestStruct struct {
		Input  string `dsgo:"input,desc=Input text"`
		Output int    `dsgo:"output,desc=Output number"`
		NoTag  string // Should be ignored
	}

	m := map[string]any{
		"Input":  "test value",
		"Output": 42,
		"NoTag":  "should be ignored",
	}

	var s TestStruct
	err := MapToStruct(m, &s)
	if err != nil {
		t.Fatalf("MapToStruct() error = %v", err)
	}

	if s.Input != "test value" {
		t.Errorf("Input = %q, want %q", s.Input, "test value")
	}

	if s.Output != 42 {
		t.Errorf("Output = %d, want %d", s.Output, 42)
	}

	if s.NoTag != "" {
		t.Errorf("NoTag = %q, want empty (should be ignored)", s.NoTag)
	}
}

func TestMapToStruct_TypeConversion(t *testing.T) {
	type TestStruct struct {
		IntField   int     `dsgo:"output"`
		FloatField float64 `dsgo:"output"`
	}

	m := map[string]any{
		"IntField":   int64(100),    // int64 -> int
		"FloatField": float32(3.14), // float32 -> float64
	}

	var s TestStruct
	err := MapToStruct(m, &s)
	if err != nil {
		t.Fatalf("MapToStruct() error = %v", err)
	}

	if s.IntField != 100 {
		t.Errorf("IntField = %d, want 100", s.IntField)
	}

	// Use approximate comparison for floats
	if s.FloatField < 3.13 || s.FloatField > 3.15 {
		t.Errorf("FloatField = %f, want ~3.14", s.FloatField)
	}
}

func TestMapToStruct_NotPointer(t *testing.T) {
	type TestStruct struct {
		Field string `dsgo:"output"`
	}

	var s TestStruct
	m := map[string]any{"Field": "value"}

	err := MapToStruct(m, s) // Not a pointer
	if err == nil {
		t.Error("MapToStruct() should return error when target is not a pointer")
	}
}

func TestMapToStruct_NilValues(t *testing.T) {
	type TestStruct struct {
		Field string `dsgo:"output"`
	}

	m := map[string]any{
		"Field": nil,
	}

	var s TestStruct
	s.Field = "initial"

	err := MapToStruct(m, &s)
	if err != nil {
		t.Fatalf("MapToStruct() error = %v", err)
	}

	// Nil values should be skipped, leaving field unchanged
	if s.Field != "initial" {
		t.Errorf("Field = %q, want %q (nil should be skipped)", s.Field, "initial")
	}
}

// Additional tests for schema.go coverage

func TestStructToSignature_WithDescription(t *testing.T) {
	type TestStruct struct {
		Input  string `dsgo:"input,desc=Input field"`
		Output string `dsgo:"output,desc=Output field"`
	}

	desc := "Custom signature description"
	sig, err := StructToSignature(reflect.TypeOf(TestStruct{}), desc)
	if err != nil {
		t.Fatalf("StructToSignature() error = %v", err)
	}

	if sig.Description != desc {
		t.Errorf("Description = %q, want %q", sig.Description, desc)
	}

	if len(sig.InputFields) != 1 {
		t.Errorf("InputFields count = %d, want 1", len(sig.InputFields))
	}

	if len(sig.OutputFields) != 1 {
		t.Errorf("OutputFields count = %d, want 1", len(sig.OutputFields))
	}
}

func TestStructToSignature_EmptyDescription(t *testing.T) {
	type TestStruct struct {
		Field string `dsgo:"input,desc=Test field"`
	}

	sig, err := StructToSignature(reflect.TypeOf(TestStruct{}), "")
	if err != nil {
		t.Fatalf("StructToSignature() error = %v", err)
	}

	if sig.Description != "" {
		t.Errorf("Description = %q, want empty", sig.Description)
	}
}

func TestStructToMap_WithUnexportedFields(t *testing.T) {
	type TestStruct struct {
		Public     string `dsgo:"input,desc=Public field"`
		unexported string `dsgo:"input,desc=Unexported"`
		NoTag      string
	}

	s := TestStruct{
		Public:     "public value",
		unexported: "unexported value",
		NoTag:      "no tag value",
	}

	m, err := StructToMap(s)
	if err != nil {
		t.Fatalf("StructToMap() error = %v", err)
	}

	if len(m) != 1 {
		t.Errorf("map length = %d, want 1 (only public tagged field)", len(m))
	}

	if m["Public"] != "public value" {
		t.Errorf("Public = %q, want %q", m["Public"], "public value")
	}

	if _, exists := m["unexported"]; exists {
		t.Error("unexported field should not be in map")
	}

	if _, exists := m["NoTag"]; exists {
		t.Error("NoTag field should not be in map")
	}
}

func TestStructToMap_AllTypes(t *testing.T) {
	type TestStruct struct {
		StrField   string         `dsgo:"input"`
		IntField   int            `dsgo:"input"`
		FloatField float64        `dsgo:"input"`
		BoolField  bool           `dsgo:"input"`
		MapField   map[string]any `dsgo:"input"`
		SliceField []string       `dsgo:"input"`
	}

	s := TestStruct{
		StrField:   "test",
		IntField:   42,
		FloatField: 3.14,
		BoolField:  true,
		MapField:   map[string]any{"key": "value"},
		SliceField: []string{"a", "b"},
	}

	m, err := StructToMap(s)
	if err != nil {
		t.Fatalf("StructToMap() error = %v", err)
	}

	if m["StrField"] != "test" {
		t.Errorf("StrField = %v, want 'test'", m["StrField"])
	}
	if m["IntField"] != 42 {
		t.Errorf("IntField = %v, want 42", m["IntField"])
	}
	if m["FloatField"] != 3.14 {
		t.Errorf("FloatField = %v, want 3.14", m["FloatField"])
	}
	if m["BoolField"] != true {
		t.Errorf("BoolField = %v, want true", m["BoolField"])
	}
}

func TestMapToStruct_MissingFields(t *testing.T) {
	type TestStruct struct {
		Field1 string `dsgo:"output"`
		Field2 int    `dsgo:"output"`
	}

	// Only provide Field1, Field2 is missing
	m := map[string]any{
		"Field1": "value1",
	}

	var s TestStruct
	s.Field2 = 99 // Set initial value

	err := MapToStruct(m, &s)
	if err != nil {
		t.Fatalf("MapToStruct() error = %v", err)
	}

	if s.Field1 != "value1" {
		t.Errorf("Field1 = %q, want %q", s.Field1, "value1")
	}

	// Field2 should remain unchanged
	if s.Field2 != 99 {
		t.Errorf("Field2 = %d, want 99 (unchanged)", s.Field2)
	}
}

func TestMapToStruct_CannotSetField(t *testing.T) {
	type TestStruct struct {
		Field string `dsgo:"output"`
	}

	m := map[string]any{
		"Field": "value",
	}

	var s TestStruct
	// This should work normally
	err := MapToStruct(m, &s)
	if err != nil {
		t.Fatalf("MapToStruct() error = %v (should succeed)", err)
	}
}

func TestMapToStruct_TypeMismatch(t *testing.T) {
	type TestStruct struct {
		Field string `dsgo:"output"`
	}

	// Provide incompatible type (complex number can't convert to string)
	m := map[string]any{
		"Field": complex(1, 2),
	}

	var s TestStruct
	err := MapToStruct(m, &s)
	if err == nil {
		t.Error("MapToStruct() should return error for incompatible types")
	}
}

func TestMapToStruct_PointerToNonStruct(t *testing.T) {
	var notStruct int
	m := map[string]any{"Field": "value"}

	err := MapToStruct(m, &notStruct)
	if err == nil {
		t.Error("MapToStruct() should return error when target is not a struct")
	}
}

func TestMapToStruct_SkipsUntaggedFields(t *testing.T) {
	type TestStruct struct {
		Tagged   string `dsgo:"output"`
		Untagged string
	}

	m := map[string]any{
		"Tagged":   "tagged value",
		"Untagged": "untagged value",
	}

	var s TestStruct
	err := MapToStruct(m, &s)
	if err != nil {
		t.Fatalf("MapToStruct() error = %v", err)
	}

	if s.Tagged != "tagged value" {
		t.Errorf("Tagged = %q, want %q", s.Tagged, "tagged value")
	}

	// Untagged field should remain zero value
	if s.Untagged != "" {
		t.Errorf("Untagged = %q, want empty (untagged fields skipped)", s.Untagged)
	}
}

func TestMapToStruct_AllTypeConversions(t *testing.T) {
	type TestStruct struct {
		Int8Field    int8    `dsgo:"output"`
		Int16Field   int16   `dsgo:"output"`
		Int32Field   int32   `dsgo:"output"`
		Int64Field   int64   `dsgo:"output"`
		Float32Field float32 `dsgo:"output"`
		Float64Field float64 `dsgo:"output"`
	}

	m := map[string]any{
		"Int8Field":    int64(8),
		"Int16Field":   int64(16),
		"Int32Field":   int64(32),
		"Int64Field":   int64(64),
		"Float32Field": float64(3.2),
		"Float64Field": float64(6.4),
	}

	var s TestStruct
	err := MapToStruct(m, &s)
	if err != nil {
		t.Fatalf("MapToStruct() error = %v", err)
	}

	if s.Int8Field != 8 {
		t.Errorf("Int8Field = %d, want 8", s.Int8Field)
	}
	if s.Int16Field != 16 {
		t.Errorf("Int16Field = %d, want 16", s.Int16Field)
	}
	if s.Int32Field != 32 {
		t.Errorf("Int32Field = %d, want 32", s.Int32Field)
	}
	if s.Int64Field != 64 {
		t.Errorf("Int64Field = %d, want 64", s.Int64Field)
	}
}

func TestStructToSignature_ParseError(t *testing.T) {
	type BadStruct struct {
		Field string `dsgo:"invalid_direction,desc=Bad"`
	}

	_, err := StructToSignature(reflect.TypeOf(BadStruct{}), "test")
	if err == nil {
		t.Error("StructToSignature() should return error for invalid tags")
	}
}

func TestMapToStruct_UnexportedFieldsSkipped(t *testing.T) {
	type TestStruct struct {
		Public     string `dsgo:"output"`
		unexported string `dsgo:"output"` //nolint:unused
	}

	m := map[string]any{
		"Public":     "public value",
		"unexported": "unexported value",
	}

	var s TestStruct
	err := MapToStruct(m, &s)
	if err != nil {
		t.Fatalf("MapToStruct() error = %v", err)
	}

	if s.Public != "public value" {
		t.Errorf("Public = %q, want 'public value'", s.Public)
	}

	// unexported field should remain zero value
	if s.unexported != "" {
		t.Errorf("unexported = %q, want empty (should be skipped)", s.unexported)
	}
}

func TestMapToStruct_ConvertibleTypes(t *testing.T) {
	type TestStruct struct {
		IntFromFloat   int     `dsgo:"output"`
		FloatFromInt   float64 `dsgo:"output"`
		IntFromFloat64 int     `dsgo:"output"`
	}

	m := map[string]any{
		"IntFromFloat":   float64(42.7),  // float64 -> int (truncates)
		"FloatFromInt":   int(100),       // int -> float64
		"IntFromFloat64": float64(99.99), // float64 -> int
	}

	var s TestStruct
	err := MapToStruct(m, &s)
	if err != nil {
		t.Fatalf("MapToStruct() error = %v", err)
	}

	if s.IntFromFloat != 42 {
		t.Errorf("IntFromFloat = %d, want 42 (truncated from 42.7)", s.IntFromFloat)
	}

	if s.FloatFromInt != 100.0 {
		t.Errorf("FloatFromInt = %f, want 100.0", s.FloatFromInt)
	}

	if s.IntFromFloat64 != 99 {
		t.Errorf("IntFromFloat64 = %d, want 99 (truncated from 99.99)", s.IntFromFloat64)
	}
}

func TestMapToStruct_NonPointerError(t *testing.T) {
	type TestStruct struct {
		Field string `dsgo:"output"`
	}

	m := map[string]any{"Field": "value"}
	var s TestStruct

	// Pass struct directly instead of pointer
	err := MapToStruct(m, s)
	if err == nil {
		t.Error("MapToStruct() should return error when target is not a pointer")
	}

	expectedMsg := "target must be a pointer to struct"
	if err.Error() != expectedMsg {
		t.Errorf("error message = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestMapToStruct_NonStructError(t *testing.T) {
	m := map[string]any{"Field": "value"}
	var notStruct string

	// Pass pointer to non-struct (string)
	err := MapToStruct(m, &notStruct)
	if err == nil {
		t.Error("MapToStruct() should return error when target is pointer to non-struct")
	}

	// Error message should indicate the actual kind
	if err == nil || err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}
