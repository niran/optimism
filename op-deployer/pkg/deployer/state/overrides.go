// File: op-deployer/pkg/deployer/state/valid_overrides.go

package state

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
)

// BuildValidOverrideKeys dynamically extracts all field names from a DeployConfig struct
func buildValidOverrideKeys() map[string]struct{} {
	validKeys := make(map[string]struct{})
	extractFieldNames(reflect.TypeOf(genesis.DeployConfig{}), "", validKeys)
	extractFieldNames(reflect.TypeOf(SuperchainProofParams{}), "", validKeys)
	extractFieldNames(reflect.TypeOf(ChainProofParams{}), "", validKeys)
	return validKeys
}

// extractFieldNames recursively extracts JSON field names from a struct type
func extractFieldNames(t reflect.Type, prefix string, result map[string]struct{}) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Handle embedded structs (fields without names)
		if field.Anonymous {
			// Recursively extract fields from the embedded struct
			extractFieldNames(field.Type, prefix, result)
			continue
		}

		// Get JSON field name
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		fieldName := strings.Split(jsonTag, ",")[0]

		// Add simple field name to support current usage pattern
		result[fieldName] = struct{}{}

		// Also add fully qualified name if we have a prefix
		if prefix != "" {
			fullPath := prefix + "." + fieldName
			result[fullPath] = struct{}{}
		}

		// Recursively process nested structs
		fieldType := field.Type
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}

		if fieldType.Kind() == reflect.Struct {
			newPrefix := fieldName
			if prefix != "" {
				newPrefix = prefix + "." + fieldName
			}
			extractFieldNames(fieldType, newPrefix, result)
		}
	}
}

// ValidateOverrides checks if all keys in the overrides map are valid override keys
func ValidateOverrides(overrides map[string]any) error {
	validKeys := buildValidOverrideKeys()

	var invalidKeys []string
	for key := range overrides {
		if _, ok := validKeys[key]; !ok {
			invalidKeys = append(invalidKeys, key)
		}
	}

	if len(invalidKeys) > 0 {
		sort.Strings(invalidKeys)
		return fmt.Errorf("invalid override keys:\n%s", strings.Join(invalidKeys, ", "))
	}

	return nil
}
