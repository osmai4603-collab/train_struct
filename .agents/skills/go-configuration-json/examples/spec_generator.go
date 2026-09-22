package config

import (
	"fmt"
	"reflect"
	"strings"
)

// =============================================================================
// Self-Documentation: Automated .env.example Template Generator
// =============================================================================

type FieldSpec struct {
	EnvKey       string
	DefaultValue string
	Description  string
}

// ExtractSpecs recursively inspects struct tags to build configuration property specs.
func ExtractSpecs(v interface{}) []FieldSpec {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	var specs []FieldSpec
	extractFromType(val.Type(), &specs)
	return specs
}

func extractFromType(t reflect.Type, specs *[]FieldSpec) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Recurse into nested structs
		if field.Type.Kind() == reflect.Struct && field.Type.Name() != "Duration" {
			extractFromType(field.Type, specs)
			continue
		}

		envKey := field.Tag.Get("env")
		if envKey == "" {
			continue
		}

		*specs = append(*specs, FieldSpec{
			EnvKey:       envKey,
			DefaultValue: field.Tag.Get("default"),
			Description:  field.Tag.Get("desc"),
		})
	}
}

// GenerateEnvTemplate outputs a clean, commented .env.example file.
func GenerateEnvTemplate(specs []FieldSpec) string {
	var sb strings.Builder
	sb.WriteString("# ==============================================================================\n")
	sb.WriteString("# Automated Environment Configuration Template (.env.example)\n")
	sb.WriteString("# ==============================================================================\n\n")

	for _, spec := range specs {
		if spec.Description != "" {
			sb.WriteString(fmt.Sprintf("# %s\n", spec.Description))
		}
		if spec.DefaultValue != "" {
			sb.WriteString(fmt.Sprintf("# Default: %s\n", spec.DefaultValue))
		}
		sb.WriteString(fmt.Sprintf("%s=%s\n\n", spec.EnvKey, spec.DefaultValue))
	}

	return sb.String()
}
