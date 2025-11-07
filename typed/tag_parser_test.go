package typed

import (
	"reflect"
	"testing"

	"github.com/assagman/dsgo/core"
)

func TestParseStructTags(t *testing.T) {
	tests := []struct {
		name      string
		input     any
		wantErr   bool
		wantCount int
	}{
		{
			name: "basic input output",
			input: struct {
				Text   string `dsgo:"input,desc=Input text"`
				Result string `dsgo:"output,desc=Result text"`
			}{},
			wantErr:   false,
			wantCount: 2,
		},
		{
			name: "enum with optional",
			input: struct {
				Text      string `dsgo:"input,desc=Text to analyze"`
				Sentiment string `dsgo:"output,enum=positive|negative|neutral"`
				Score     int    `dsgo:"output,optional,desc=Confidence score"`
			}{},
			wantErr:   false,
			wantCount: 3,
		},
		{
			name: "skip unexported and untagged",
			input: struct {
				Public  string `dsgo:"input,desc=Public field"`
				private string `dsgo:"input,desc=Private field"` // Should be skipped
				NoTag   string // Should be skipped
			}{},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name: "with alias",
			input: struct {
				Result string `dsgo:"output,enum=positive|negative,alias:pos=positive,alias:neg=negative"`
			}{},
			wantErr:   false,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ := reflect.TypeOf(tt.input)
			fields, err := ParseStructTags(typ)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStructTags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(fields) != tt.wantCount {
				t.Errorf("ParseStructTags() got %d fields, want %d", len(fields), tt.wantCount)
			}
		})
	}
}

func TestParseFieldTag(t *testing.T) {
	tests := []struct {
		name         string
		tag          string
		fieldType    reflect.Type
		wantInput    bool
		wantOutput   bool
		wantOptional bool
		wantDesc     string
		wantClasses  []string
		wantErr      bool
	}{
		{
			name:       "input field",
			tag:        "input,desc=User input",
			fieldType:  reflect.TypeOf(""),
			wantInput:  true,
			wantOutput: false,
			wantDesc:   "User input",
		},
		{
			name:        "output enum",
			tag:         "output,enum=yes|no|maybe",
			fieldType:   reflect.TypeOf(""),
			wantInput:   false,
			wantOutput:  true,
			wantClasses: []string{"yes", "no", "maybe"},
		},
		{
			name:         "optional output",
			tag:          "output,optional,desc=Optional result",
			fieldType:    reflect.TypeOf(0),
			wantOutput:   true,
			wantOptional: true,
			wantDesc:     "Optional result",
		},
		{
			name:      "invalid tag",
			tag:       "invalid,desc=test",
			fieldType: reflect.TypeOf(""),
			wantErr:   true,
		},
		{
			name:      "empty tag",
			tag:       "",
			fieldType: reflect.TypeOf(""),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := parseFieldTag("TestField", tt.fieldType, tt.tag)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseFieldTag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if info.IsInput != tt.wantInput {
				t.Errorf("IsInput = %v, want %v", info.IsInput, tt.wantInput)
			}
			if info.IsOutput != tt.wantOutput {
				t.Errorf("IsOutput = %v, want %v", info.IsOutput, tt.wantOutput)
			}
			if info.Optional != tt.wantOptional {
				t.Errorf("Optional = %v, want %v", info.Optional, tt.wantOptional)
			}
			if info.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", info.Description, tt.wantDesc)
			}
			if tt.wantClasses != nil {
				if len(info.Classes) != len(tt.wantClasses) {
					t.Errorf("Classes length = %d, want %d", len(info.Classes), len(tt.wantClasses))
				}
			}
		})
	}
}

func TestInferFieldType(t *testing.T) {
	tests := []struct {
		name    string
		goType  reflect.Type
		classes []string
		want    core.FieldType
	}{
		{
			name:   "string",
			goType: reflect.TypeOf(""),
			want:   core.FieldTypeString,
		},
		{
			name:   "int",
			goType: reflect.TypeOf(0),
			want:   core.FieldTypeInt,
		},
		{
			name:   "float64",
			goType: reflect.TypeOf(0.0),
			want:   core.FieldTypeFloat,
		},
		{
			name:   "bool",
			goType: reflect.TypeOf(true),
			want:   core.FieldTypeBool,
		},
		{
			name:   "map",
			goType: reflect.TypeOf(map[string]any{}),
			want:   core.FieldTypeJSON,
		},
		{
			name:    "string with enum becomes class",
			goType:  reflect.TypeOf(""),
			classes: []string{"a", "b"},
			want:    core.FieldTypeClass,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferFieldType(tt.goType, tt.classes)
			if got != tt.want {
				t.Errorf("inferFieldType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want []string
	}{
		{
			name: "simple split",
			tag:  "input,desc=test",
			want: []string{"input", "desc=test"},
		},
		{
			name: "with enum",
			tag:  "output,enum=a|b|c,desc=test",
			want: []string{"output", "enum=a|b|c", "desc=test"},
		},
		{
			name: "empty parts filtered",
			tag:  "input,,desc=test",
			want: []string{"input", "desc=test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitTag(tt.tag)
			if len(got) != len(tt.want) {
				t.Errorf("splitTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Additional tests for tag_parser.go coverage

func TestParseStructTags_EmptyStruct(t *testing.T) {
	type EmptyStruct struct{}

	fields, err := ParseStructTags(reflect.TypeOf(EmptyStruct{}))
	if err != nil {
		t.Fatalf("ParseStructTags() error = %v", err)
	}

	if len(fields) != 0 {
		t.Errorf("fields count = %d, want 0", len(fields))
	}
}

func TestParseStructTags_OnlyUnexportedFields(t *testing.T) {
	type TestStruct struct {
		unexported1 string `dsgo:"input,desc=Unexported"`       //nolint:unused
		unexported2 int    `dsgo:"output,desc=Also unexported"` //nolint:unused
	}

	fields, err := ParseStructTags(reflect.TypeOf(TestStruct{}))
	if err != nil {
		t.Fatalf("ParseStructTags() error = %v", err)
	}

	if len(fields) != 0 {
		t.Errorf("fields count = %d, want 0 (all unexported)", len(fields))
	}
}

func TestParseStructTags_MixedExportedUnexported(t *testing.T) {
	type TestStruct struct {
		Public     string `dsgo:"input,desc=Public"`
		unexported string `dsgo:"input,desc=Unexported"` //nolint:unused
		NoTag      string
	}

	fields, err := ParseStructTags(reflect.TypeOf(TestStruct{}))
	if err != nil {
		t.Fatalf("ParseStructTags() error = %v", err)
	}

	if len(fields) != 1 {
		t.Errorf("fields count = %d, want 1", len(fields))
	}

	if fields[0].Name != "Public" {
		t.Errorf("field name = %s, want Public", fields[0].Name)
	}
}

func TestParseFieldTag_WithAllOptions(t *testing.T) {
	tag := "output,optional,desc=Complex field with description,enum=a|b|c,alias:x=a,alias:y=b"
	info, err := parseFieldTag("ComplexField", reflect.TypeOf(""), tag)
	if err != nil {
		t.Fatalf("parseFieldTag() error = %v", err)
	}

	if !info.IsOutput {
		t.Error("IsOutput should be true")
	}
	if info.IsInput {
		t.Error("IsInput should be false")
	}
	if !info.Optional {
		t.Error("Optional should be true")
	}
	if info.Description != "Complex field with description" {
		t.Errorf("Description = %q, want 'Complex field with description'", info.Description)
	}
	if len(info.Classes) != 3 {
		t.Errorf("Classes count = %d, want 3", len(info.Classes))
	}
	if len(info.ClassAliases) != 2 {
		t.Errorf("ClassAliases count = %d, want 2", len(info.ClassAliases))
	}
	if info.ClassAliases["x"] != "a" {
		t.Errorf("Alias x = %s, want 'a'", info.ClassAliases["x"])
	}
	if info.ClassAliases["y"] != "b" {
		t.Errorf("Alias y = %s, want 'b'", info.ClassAliases["y"])
	}
}

func TestParseFieldTag_UnknownOptions(t *testing.T) {
	// Unknown options should be ignored for forward compatibility
	tag := "input,unknown_option,desc=Test,another_unknown"
	info, err := parseFieldTag("TestField", reflect.TypeOf(""), tag)
	if err != nil {
		t.Fatalf("parseFieldTag() error = %v", err)
	}

	if !info.IsInput {
		t.Error("IsInput should be true")
	}
	if info.Description != "Test" {
		t.Errorf("Description = %q, want 'Test'", info.Description)
	}
}

func TestInferFieldType_AllIntTypes(t *testing.T) {
	tests := []struct {
		name   string
		goType reflect.Type
		want   core.FieldType
	}{
		{"int", reflect.TypeOf(int(0)), core.FieldTypeInt},
		{"int8", reflect.TypeOf(int8(0)), core.FieldTypeInt},
		{"int16", reflect.TypeOf(int16(0)), core.FieldTypeInt},
		{"int32", reflect.TypeOf(int32(0)), core.FieldTypeInt},
		{"int64", reflect.TypeOf(int64(0)), core.FieldTypeInt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferFieldType(tt.goType, nil)
			if got != tt.want {
				t.Errorf("inferFieldType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInferFieldType_AllFloatTypes(t *testing.T) {
	tests := []struct {
		name   string
		goType reflect.Type
		want   core.FieldType
	}{
		{"float32", reflect.TypeOf(float32(0)), core.FieldTypeFloat},
		{"float64", reflect.TypeOf(float64(0)), core.FieldTypeFloat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferFieldType(tt.goType, nil)
			if got != tt.want {
				t.Errorf("inferFieldType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInferFieldType_SliceType(t *testing.T) {
	sliceType := reflect.TypeOf([]string{})
	got := inferFieldType(sliceType, nil)
	if got != core.FieldTypeJSON {
		t.Errorf("inferFieldType(slice) = %v, want JSON", got)
	}
}

func TestInferFieldType_StructType(t *testing.T) {
	type TestStruct struct {
		Field string
	}
	structType := reflect.TypeOf(TestStruct{})
	got := inferFieldType(structType, nil)
	if got != core.FieldTypeJSON {
		t.Errorf("inferFieldType(struct) = %v, want JSON", got)
	}
}

func TestInferFieldType_DefaultFallback(t *testing.T) {
	// Use an unusual type that should fall back to string
	ptrType := reflect.TypeOf((*int)(nil))
	got := inferFieldType(ptrType, nil)
	if got != core.FieldTypeString {
		t.Errorf("inferFieldType(unusual type) = %v, want String (fallback)", got)
	}
}

func TestSplitTag_WithQuotes(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want int
	}{
		{
			name: "single quoted value",
			tag:  "input,desc='quoted value'",
			want: 2,
		},
		{
			name: "double quoted value",
			tag:  `input,desc="quoted value"`,
			want: 2,
		},
		{
			name: "comma inside quotes",
			tag:  `input,desc="value,with,commas"`,
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitTag(tt.tag)
			if len(got) != tt.want {
				t.Errorf("splitTag() = %v parts, want %d", len(got), tt.want)
			}
		})
	}
}

func TestSplitTag_EmptyTag(t *testing.T) {
	got := splitTag("")
	if len(got) != 0 {
		t.Errorf("splitTag('') = %d parts, want 0", len(got))
	}
}

func TestSplitTag_OnlyCommas(t *testing.T) {
	got := splitTag(",,,")
	if len(got) != 0 {
		t.Errorf("splitTag(',,,') = %d parts, want 0 (all empty)", len(got))
	}
}

func TestSplitTag_TrailingComma(t *testing.T) {
	got := splitTag("input,output,")
	// Should have 2 parts (trailing comma ignored)
	if len(got) != 2 {
		t.Errorf("splitTag() = %d parts, want 2", len(got))
	}
}

func TestParseFieldTag_AliasWithoutEquals(t *testing.T) {
	// Malformed alias (no =) should be ignored
	tag := "output,alias:noequals,desc=Test"
	info, err := parseFieldTag("Field", reflect.TypeOf(""), tag)
	if err != nil {
		t.Fatalf("parseFieldTag() error = %v", err)
	}

	// Should have 0 aliases since the format was wrong
	if len(info.ClassAliases) != 0 {
		t.Errorf("ClassAliases count = %d, want 0 (malformed alias ignored)", len(info.ClassAliases))
	}
}

func TestParseFieldTag_MultipleAliases(t *testing.T) {
	tag := "output,enum=positive|negative,alias:pos=positive,alias:neg=negative,alias:p=positive"
	info, err := parseFieldTag("Sentiment", reflect.TypeOf(""), tag)
	if err != nil {
		t.Fatalf("parseFieldTag() error = %v", err)
	}

	if len(info.ClassAliases) != 3 {
		t.Errorf("ClassAliases count = %d, want 3", len(info.ClassAliases))
	}

	expectedAliases := map[string]string{
		"pos": "positive",
		"neg": "negative",
		"p":   "positive",
	}

	for alias, expected := range expectedAliases {
		if info.ClassAliases[alias] != expected {
			t.Errorf("ClassAliases[%s] = %s, want %s", alias, info.ClassAliases[alias], expected)
		}
	}
}

func TestParseFieldTag_EnumWithSingleValue(t *testing.T) {
	tag := "output,enum=onlyvalue"
	info, err := parseFieldTag("Field", reflect.TypeOf(""), tag)
	if err != nil {
		t.Fatalf("parseFieldTag() error = %v", err)
	}

	if len(info.Classes) != 1 {
		t.Errorf("Classes count = %d, want 1", len(info.Classes))
	}
	if info.Classes[0] != "onlyvalue" {
		t.Errorf("Classes[0] = %s, want 'onlyvalue'", info.Classes[0])
	}
	if info.Type != core.FieldTypeClass {
		t.Errorf("Type = %v, want FieldTypeClass", info.Type)
	}
}

func TestParseStructTags_InvalidTag(t *testing.T) {
	type InvalidStruct struct {
		BadField string `dsgo:"invalid_direction,desc=Bad"`
	}

	_, err := ParseStructTags(reflect.TypeOf(InvalidStruct{}))
	if err == nil {
		t.Error("ParseStructTags() should return error for invalid tag direction")
	}
}

func TestParseStructTags_NotStruct(t *testing.T) {
	notStruct := "not a struct"
	_, err := ParseStructTags(reflect.TypeOf(notStruct))
	if err == nil {
		t.Error("ParseStructTags() should return error for non-struct type")
	}
}
