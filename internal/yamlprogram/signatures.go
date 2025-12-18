package yamlprogram

import (
	"fmt"

	"github.com/assagman/dsgo"
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

func convertSignature(name string, spec SignatureSpec) (*dsgo.Signature, error) {
	sig := dsgo.NewSignature(spec.Desc)

	for fieldName, f := range spec.In {
		ft, classes, err := mapFieldType(f)
		if err != nil {
			return nil, fmt.Errorf("signature %q in.%s: %w", name, fieldName, err)
		}

		if f.Optional {
			sig.AddOptionalInput(fieldName, ft, f.Desc)
		} else {
			sig.AddInput(fieldName, ft, f.Desc)
		}
		_ = classes // inputs don't use classes in DSGo
	}

	for fieldName, f := range spec.Out {
		ft, classes, err := mapFieldType(f)
		if err != nil {
			return nil, fmt.Errorf("signature %q out.%s: %w", name, fieldName, err)
		}

		if ft == dsgo.FieldTypeClass {
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

func mapFieldType(f FieldSpec) (dsgo.FieldType, []string, error) {
	switch f.Type {
	case "string":
		return dsgo.FieldTypeString, nil, nil
	case "int":
		return dsgo.FieldTypeInt, nil, nil
	case "float":
		return dsgo.FieldTypeFloat, nil, nil
	case "bool":
		return dsgo.FieldTypeBool, nil, nil
	case "json":
		return dsgo.FieldTypeJSON, nil, nil
	case "image":
		return dsgo.FieldTypeImage, nil, nil
	case "datetime":
		return dsgo.FieldTypeDatetime, nil, nil
	case "enum":
		if len(f.Values) == 0 {
			return dsgo.FieldTypeString, nil, fmt.Errorf("enum values is empty")
		}
		return dsgo.FieldTypeClass, f.Values, nil
	default:
		return dsgo.FieldTypeString, nil, fmt.Errorf("unknown field type %q", f.Type)
	}
}
