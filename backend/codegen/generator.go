package codegen

import (
	"fmt"
	"strings"
	"unicode"

	"api-mocker/models"
)

type TypeScriptGenerator struct{}

func NewTypeScript() *TypeScriptGenerator {
	return &TypeScriptGenerator{}
}

func (g *TypeScriptGenerator) Generate(modelsList []ModelDef) string {
	var sb strings.Builder

	for _, m := range modelsList {
		sb.WriteString(fmt.Sprintf("export interface %s {\n", m.Name))
		for _, field := range m.Fields {
			tsType := g.toTSType(field)
			optional := ""
			if !field.Required {
				optional = "?"
			}
			sb.WriteString(fmt.Sprintf("  %s%s: %s;\n", g.toCamelCase(field.Name), optional, tsType))
		}
		sb.WriteString("}\n\n")
	}

	return sb.String()
}

func (g *TypeScriptGenerator) toTSType(field FieldDef) string {
	if field.Ref != "" {
		return field.Ref
	}
	if len(field.Enum) > 0 {
		return strings.Join(field.Enum, " | ")
	}

	switch field.Type {
	case "string":
		return "string"
	case "number", "integer":
		return "number"
	case "boolean":
		return "boolean"
	case "array":
		if len(field.Children) > 0 {
			return g.toTSType(field.Children[0]) + "[]"
		}
		return "any[]"
	case "object":
		if len(field.Children) > 0 {
			var parts []string
			for _, c := range field.Children {
				optional := ""
				if !c.Required {
					optional = "?"
				}
				parts = append(parts, fmt.Sprintf("%s%s: %s", g.toCamelCase(c.Name), optional, g.toTSType(c)))
			}
			return "{ " + strings.Join(parts, "; ") + " }"
		}
		return "Record<string, any>"
	default:
		return "any"
	}
}

func (g *TypeScriptGenerator) toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = string(unicode.ToUpper(rune(parts[i][0]))) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

type GoGenerator struct{}

func NewGo() *GoGenerator {
	return &GoGenerator{}
}

func (g *GoGenerator) Generate(modelsList []ModelDef) string {
	var sb strings.Builder

	for _, m := range modelsList {
		sb.WriteString(fmt.Sprintf("type %s struct {\n", g.toPascalCase(m.Name)))
		for _, field := range m.Fields {
			goType := g.toGoType(field)
			jsonTag := field.Name
			if !field.Required {
				jsonTag += ",omitempty"
			}
			sb.WriteString(fmt.Sprintf("\t%s %s `json:\"%s\"`\n", g.toPascalCase(field.Name), goType, jsonTag))
		}
		sb.WriteString("}\n\n")
	}

	return sb.String()
}

func (g *GoGenerator) toGoType(field FieldDef) string {
	if field.Ref != "" {
		return g.toPascalCase(field.Ref)
	}

	switch field.Type {
	case "string":
		return "string"
	case "number":
		return "float64"
	case "integer":
		return "int"
	case "boolean":
		return "bool"
	case "array":
		if len(field.Children) > 0 {
			return "[]" + g.toGoType(field.Children[0])
		}
		return "[]interface{}"
	case "object":
		return "map[string]interface{}"
	default:
		return "interface{}"
	}
}

func (g *GoGenerator) toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		if len(parts[i]) > 0 {
			parts[i] = string(unicode.ToUpper(rune(parts[i][0]))) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

type PythonGenerator struct{}

func NewPython() *PythonGenerator {
	return &PythonGenerator{}
}

func (g *PythonGenerator) Generate(modelsList []ModelDef) string {
	var sb strings.Builder

	sb.WriteString("from pydantic import BaseModel\n")
	sb.WriteString("from typing import Optional, List, Any\n\n")

	for _, m := range modelsList {
		sb.WriteString(fmt.Sprintf("class %s(BaseModel):\n", g.toPascalCase(m.Name)))
		if len(m.Fields) == 0 {
			sb.WriteString("\tpass\n\n")
			continue
		}
		for _, field := range m.Fields {
			pyType := g.toPythonType(field)
			if field.Required {
				sb.WriteString(fmt.Sprintf("\t%s: %s\n", g.toSnakeCase(field.Name), pyType))
			} else {
				sb.WriteString(fmt.Sprintf("\t%s: Optional[%s] = None\n", g.toSnakeCase(field.Name), pyType))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (g *PythonGenerator) toPythonType(field FieldDef) string {
	if field.Ref != "" {
		return g.toPascalCase(field.Ref)
	}

	switch field.Type {
	case "string":
		return "str"
	case "number":
		return "float"
	case "integer":
		return "int"
	case "boolean":
		return "bool"
	case "array":
		if len(field.Children) > 0 {
			return fmt.Sprintf("List[%s]", g.toPythonType(field.Children[0]))
		}
		return "List[Any]"
	case "object":
		return "dict"
	default:
		return "Any"
	}
}

func (g *PythonGenerator) toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		if len(parts[i]) > 0 {
			parts[i] = string(unicode.ToUpper(rune(parts[i][0]))) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

func (g *PythonGenerator) toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, unicode.ToLower(r))
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

type ModelDef struct {
	Name   string
	Fields []FieldDef
}

type FieldDef struct {
	Name     string
	Type     string
	Required bool
	Ref      string
	Enum     []string
	Children []FieldDef
}

func ConvertBodyFields(fields []models.BodyField) []FieldDef {
	result := make([]FieldDef, 0, len(fields))
	for _, f := range fields {
		fd := FieldDef{
			Name:     f.Name,
			Type:     f.Type,
			Required: f.Required,
			Ref:      f.Ref,
			Enum:     f.Enum,
		}
		if len(f.Children) > 0 {
			fd.Children = ConvertBodyFields(f.Children)
		}
		result = append(result, fd)
	}
	return result
}
