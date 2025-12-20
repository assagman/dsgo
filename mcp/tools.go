package mcp

import (
	"context"
	"os"
	"slices"

	"github.com/assagman/dsgo/core"
)

// ConvertMCPToolsToDSGo converts MCP tool schemas to DSGo tools.
func ConvertMCPToolsToDSGo(schemas []MCPToolSchema, client *Client) []core.Tool {
	var tools []core.Tool
	for _, schema := range schemas {
		tools = append(tools, convertTool(schema, client))
	}
	return tools
}

// inferParameterType infers parameter type and element type from an MCP JSON Schema property.
func inferParameterType(defMap map[string]any) (paramType string, elementType string) {
	// 1. Direct `type`
	if typeVal, ok := defMap["type"].(string); ok {
		// If type is "array", get the element type from items
		if typeVal == "array" {
			if items, ok := defMap["items"].(map[string]any); ok {
				if elemType, ok := items["type"].(string); ok {
					return typeVal, elemType
				}
			}
		}
		return typeVal, ""
	}

	// 2. Union types (`anyOf` / `oneOf` / `allOf`)
	for _, field := range []string{"anyOf", "oneOf", "allOf"} {
		if unionVal, ok := defMap[field].([]any); ok {
			var foundTypes []string
			var arrayItemTypes []string

			for _, subSchema := range unionVal {
				if subMap, ok := subSchema.(map[string]any); ok {
					if typeVal, ok := subMap["type"].(string); ok {
						switch typeVal {
						case "string", "boolean", "number", "integer":
							foundTypes = append(foundTypes, typeVal)
						case "array":
							foundTypes = append(foundTypes, typeVal)
							// Get the element type from array items
							if items, ok := subMap["items"].(map[string]any); ok {
								if elemType, ok := items["type"].(string); ok {
									arrayItemTypes = append(arrayItemTypes, elemType)
								}
							}
						case "object":
							foundTypes = append(foundTypes, "json")
						}
					}
				}
			}

			// Special-case patterns like Jina's: string OR array of string -> treat as array with string elements
			hasString := slices.Contains(foundTypes, "string")
			hasArray := slices.Contains(foundTypes, "array")
			arrayElementType := ""
			if len(arrayItemTypes) > 0 {
				arrayElementType = arrayItemTypes[0]
			}

			if hasString && hasArray && arrayElementType == "string" {
				return "array", "string"
			}

			// If any subschema has "type": "array", prefer "array"
			for _, t := range foundTypes {
				if t == "array" && len(arrayItemTypes) > 0 {
					return "array", arrayItemTypes[0]
				}
			}

			// If at least one primitive type appears, pick the first one
			if len(foundTypes) > 0 {
				return foundTypes[0], ""
			}
		}
	}

	// 3. Fallback for unknown/complex schemas
	// If the schema obviously looks like an object (has nested `properties`), treat as `"json"`
	if _, hasProps := defMap["properties"]; hasProps {
		return "json", ""
	}

	// Otherwise, default to "string"
	return "string", ""
}

func convertTool(schema MCPToolSchema, client *Client) core.Tool {
	// Create the tool function
	toolFunc := func(ctx context.Context, args map[string]any) (any, error) {
		return client.CallTool(ctx, schema.Name, args)
	}

	// Create the DSGo tool
	t := core.NewTool(schema.Name, schema.Description, toolFunc)

	// Add parameters
	if schema.InputSchema.Properties != nil {
		for paramName, paramDef := range schema.InputSchema.Properties {
			defMap, ok := paramDef.(map[string]any)
			if !ok {
				continue
			}

			paramDesc, _ := defMap["description"].(string)

			// Check if required
			required := slices.Contains(schema.InputSchema.Required, paramName)

			// Use the inference helper to get parameter type and element type
			paramType, elementType := inferParameterType(defMap)

			// Never leave ToolParameter.Type empty; enforce that paramType is always a non-empty string
			if paramType == "" {
				paramType = "string"
				// Log a debug message if we had to fall back
				if os.Getenv("DSGO_DEBUG_PARSE") != "" {
					println("DSGO_DEBUG: MCP tool parameter", paramName, "had to fallback to string type")
				}
			}

			switch paramType {
			case "array":
				t.AddArrayParameter(paramName, paramDesc, elementType, required)
			case "object", "json":
				t.AddParameter(paramName, "json", paramDesc, required)
			default:
				t.AddParameter(paramName, paramType, paramDesc, required)
			}
		}
	}

	return *t
}
