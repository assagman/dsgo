package yamlprogram

import (
	"fmt"

	"github.com/assagman/dsgo/internal/core"
)

func (b *Builder) buildSignatures() error {
	for name, spec := range b.spec.Signatures {
		sig, err := convertSignature(name, spec)
		if err != nil {
			return err
		}
		b.signatures[name] = sig
	}
	return nil
}

func convertSignature(name string, spec SignatureSpec) (*core.Signature, error) {
	sig := core.NewSignature(spec.Desc)

	for fieldName, f := range spec.In {
		ft, _, err := mapFieldType(f)
		if err != nil {
			return nil, fmt.Errorf("signature %q in.%s: %w", name, fieldName, err)
		}

		if f.Optional {
			sig.AddOptionalInput(fieldName, ft, f.Desc)
		} else {
			sig.AddInput(fieldName, ft, f.Desc)
		}
	}

	for fieldName, f := range spec.Out {
		ft, classes, err := mapFieldType(f)
		if err != nil {
			return nil, fmt.Errorf("signature %q out.%s: %w", name, fieldName, err)
		}

		if ft == core.FieldTypeClass {
			sig.AddClassOutput(fieldName, classes, f.Desc)
			continue
		}

		if f.Optional {
			sig.AddOptionalOutput(fieldName, ft, f.Desc)
		} else {
			sig.AddOutput(fieldName, ft, f.Desc)
		}
	}

	return sig, nil
}

func mapFieldType(f FieldSpec) (core.FieldType, []string, error) {
	switch f.Type {
	case "string":
		return core.FieldTypeString, nil, nil
	case "int":
		return core.FieldTypeInt, nil, nil
	case "float":
		return core.FieldTypeFloat, nil, nil
	case "bool":
		return core.FieldTypeBool, nil, nil
	case "json":
		return core.FieldTypeJSON, nil, nil
	case "image":
		return core.FieldTypeImage, nil, nil
	case "datetime":
		return core.FieldTypeDatetime, nil, nil
	case "enum":
		if len(f.Values) == 0 {
			return core.FieldTypeString, nil, fmt.Errorf("enum values is empty")
		}
		return core.FieldTypeClass, f.Values, nil
	default:
		return core.FieldTypeString, nil, fmt.Errorf("unknown field type %q", f.Type)
	}
}
