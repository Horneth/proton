package template

import (
	"google.golang.org/protobuf/reflect/protoreflect"
)

// GenerateJSONTemplate recursively creates a map representing a JSON template for a message
func GenerateJSONTemplate(md protoreflect.MessageDescriptor) map[string]interface{} {
	template := make(map[string]interface{})
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		template[string(fd.Name())] = getExampleValue(fd)
	}
	return template
}

func getExampleValue(fd protoreflect.FieldDescriptor) interface{} {
	if fd.IsList() {
		return []interface{}{getSingleExampleValue(fd)}
	}
	if fd.IsMap() {
		return map[string]interface{}{
			"key": getSingleExampleValue(fd.MapValue()),
		}
	}
	return getSingleExampleValue(fd)
}

func getSingleExampleValue(fd protoreflect.FieldDescriptor) interface{} {
	switch fd.Kind() {
	case protoreflect.StringKind:
		return "example_string"
	case protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		return 0
	case protoreflect.BoolKind:
		return false
	case protoreflect.EnumKind:
		if fd.Enum().Values().Len() > 0 {
			return string(fd.Enum().Values().Get(0).Name())
		}
		return "UNKNOWN"
	case protoreflect.MessageKind:
		return GenerateJSONTemplate(fd.Message())
	default:
		return nil
	}
}
