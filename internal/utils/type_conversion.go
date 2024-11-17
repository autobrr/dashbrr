// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package utils

import (
	"fmt"
	"reflect"
	"runtime"

	"github.com/rs/zerolog/log"
)

// Error codes for type conversion
const (
	ErrCodeNilInput         = "TYPE_001"
	ErrCodeTypeMismatch     = "TYPE_002"
	ErrCodeNilMap           = "TYPE_003"
	ErrCodeNilSlice         = "TYPE_004"
	ErrCodeStructConversion = "TYPE_005"
)

// ConversionError represents a type conversion error with detailed context
type ConversionError struct {
	Code       string
	Message    string
	InputType  string
	TargetType string
	StackTrace string
}

func (e *ConversionError) Error() string {
	return fmt.Sprintf("[%s] %s (input: %s, target: %s)", e.Code, e.Message, e.InputType, e.TargetType)
}

// getStackTrace returns the current stack trace
func getStackTrace() string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	var trace string
	for {
		frame, more := frames.Next()
		trace += fmt.Sprintf("\n\t%s:%d %s", frame.File, frame.Line, frame.Function)
		if !more {
			break
		}
	}
	return trace
}

// newConversionError creates a new ConversionError with stack trace
func newConversionError(code string, msg string, input interface{}, targetType reflect.Type) *ConversionError {
	return &ConversionError{
		Code:       code,
		Message:    msg,
		InputType:  GetTypeString(input),
		TargetType: targetType.String(),
		StackTrace: getStackTrace(),
	}
}

// SafeConvert performs a type-safe conversion with comprehensive error handling.
// It attempts to convert the input interface{} to the target type T.
// If the input is nil, it returns the zero value of T and an error.
// If the input is already of type T, it returns the value directly.
// Otherwise, it returns the zero value of T and an error with detailed type information.
//
// Parameters:
//   - input: The value to convert
//
// Returns:
//   - T: The converted value or zero value if conversion fails
//   - error: Detailed ConversionError if conversion fails
func SafeConvert[T any](input interface{}) (T, error) {
	var zero T
	if input == nil {
		err := newConversionError(ErrCodeNilInput, "input is nil", input, reflect.TypeOf(zero))
		log.Error().
			Str("error_code", err.Code).
			Str("stack_trace", err.StackTrace).
			Msg(err.Message)
		return zero, err
	}

	// Check if input is already of type T
	if val, ok := input.(T); ok {
		return val, nil
	}

	err := newConversionError(
		ErrCodeTypeMismatch,
		fmt.Sprintf("cannot convert %v to target type", input),
		input,
		reflect.TypeOf(zero),
	)

	log.Error().
		Str("error_code", err.Code).
		Str("input_type", err.InputType).
		Str("target_type", err.TargetType).
		Str("stack_trace", err.StackTrace).
		Msg(err.Message)

	return zero, err
}

// SafeConvertWithDefault performs a type-safe conversion with a fallback default value.
// It uses SafeConvert internally and returns the default value if conversion fails.
// The conversion attempt is logged at debug level if it fails.
//
// Parameters:
//   - input: The value to convert
//   - defaultValue: The value to return if conversion fails
//
// Returns:
//   - T: The converted value or defaultValue if conversion fails
func SafeConvertWithDefault[T any](input interface{}, defaultValue T) T {
	result, err := SafeConvert[T](input)
	if err != nil {
		log.Debug().
			Str("error_code", err.(*ConversionError).Code).
			Interface("input", input).
			Interface("default", defaultValue).
			Str("stack_trace", err.(*ConversionError).StackTrace).
			Msg("Type conversion failed, using default value")
		return defaultValue
	}
	return result
}

// SafeMapConvert converts a map[string]interface{} to a map[string]T.
// It attempts to convert each value in the input map to type T.
// If any conversion fails, it returns nil and a detailed error.
// If the input map is nil, it returns nil and an error.
//
// Parameters:
//   - input: The map to convert
//
// Returns:
//   - map[string]T: The converted map
//   - error: Detailed ConversionError if any conversion fails
func SafeMapConvert[T any](input map[string]interface{}) (map[string]T, error) {
	if input == nil {
		err := newConversionError(ErrCodeNilMap, "input map is nil", input, reflect.TypeOf(map[string]T{}))
		log.Error().
			Str("error_code", err.Code).
			Str("stack_trace", err.StackTrace).
			Msg(err.Message)
		return nil, err
	}

	result := make(map[string]T, len(input))
	for key, value := range input {
		converted, err := SafeConvert[T](value)
		if err != nil {
			convErr := err.(*ConversionError)
			log.Error().
				Str("error_code", convErr.Code).
				Str("key", key).
				Str("input_type", convErr.InputType).
				Str("target_type", convErr.TargetType).
				Str("stack_trace", convErr.StackTrace).
				Msgf("Failed to convert map value for key '%s'", key)
			return nil, fmt.Errorf("failed to convert value for key '%s': %w", key, err)
		}
		result[key] = converted
	}
	return result, nil
}

// SafeSliceConvert converts a []interface{} to []T.
// It attempts to convert each element in the input slice to type T.
// If any conversion fails, it returns nil and a detailed error.
// If the input slice is nil, it returns nil and an error.
//
// Parameters:
//   - input: The slice to convert
//
// Returns:
//   - []T: The converted slice
//   - error: Detailed ConversionError if any conversion fails
func SafeSliceConvert[T any](input []interface{}) ([]T, error) {
	if input == nil {
		err := newConversionError(ErrCodeNilSlice, "input slice is nil", input, reflect.TypeOf([]T{}))
		log.Error().
			Str("error_code", err.Code).
			Str("stack_trace", err.StackTrace).
			Msg(err.Message)
		return nil, err
	}

	result := make([]T, len(input))
	for i, value := range input {
		converted, err := SafeConvert[T](value)
		if err != nil {
			convErr := err.(*ConversionError)
			log.Error().
				Str("error_code", convErr.Code).
				Int("index", i).
				Str("input_type", convErr.InputType).
				Str("target_type", convErr.TargetType).
				Str("stack_trace", convErr.StackTrace).
				Msgf("Failed to convert slice value at index %d", i)
			return nil, fmt.Errorf("failed to convert value at index %d: %w", i, err)
		}
		result[i] = converted
	}
	return result, nil
}

// SafeStructConvert safely converts between struct types by matching field names.
// It uses reflection to copy fields from the input struct to a new struct of type T.
// Only fields with matching names and compatible types are converted.
// Handles both value and pointer input types.
//
// Parameters:
//   - input: The struct to convert (can be a value or pointer)
//
// Returns:
//   - T: The converted struct
//   - error: Detailed ConversionError if conversion fails
func SafeStructConvert[T any](input interface{}) (T, error) {
	var zero T
	if input == nil {
		err := newConversionError(ErrCodeStructConversion, "input is nil", input, reflect.TypeOf(zero))
		log.Error().
			Str("error_code", err.Code).
			Str("stack_trace", err.StackTrace).
			Msg(err.Message)
		return zero, err
	}

	// Get reflect values
	inputVal := reflect.ValueOf(input)
	outputVal := reflect.New(reflect.TypeOf(zero)).Elem()

	// Handle pointers
	if inputVal.Kind() == reflect.Ptr {
		if inputVal.IsNil() {
			err := newConversionError(ErrCodeStructConversion, "input pointer is nil", input, reflect.TypeOf(zero))
			log.Error().
				Str("error_code", err.Code).
				Str("stack_trace", err.StackTrace).
				Msg(err.Message)
			return zero, err
		}
		inputVal = inputVal.Elem()
	}

	// Ensure we're working with structs
	if inputVal.Kind() != reflect.Struct {
		err := newConversionError(
			ErrCodeStructConversion,
			fmt.Sprintf("input must be a struct, got %v", inputVal.Kind()),
			input,
			reflect.TypeOf(zero),
		)
		log.Error().
			Str("error_code", err.Code).
			Str("input_kind", inputVal.Kind().String()).
			Str("stack_trace", err.StackTrace).
			Msg(err.Message)
		return zero, err
	}

	// Get type information for input and output
	inputType := inputVal.Type()
	outputType := outputVal.Type()

	// Track conversion issues for logging
	var conversionIssues []string

	// Iterate through output fields
	for i := 0; i < outputType.NumField(); i++ {
		outputField := outputType.Field(i)

		// Find corresponding input field
		if inputField, found := inputType.FieldByName(outputField.Name); found {
			// Get field values
			inputFieldVal := inputVal.FieldByName(outputField.Name)
			outputFieldVal := outputVal.Field(i)

			// Skip if input field is not valid
			if !inputFieldVal.IsValid() {
				conversionIssues = append(conversionIssues,
					fmt.Sprintf("field '%s' is invalid", outputField.Name))
				continue
			}

			// Try to convert and set the field
			if inputFieldVal.Type().ConvertibleTo(outputField.Type) {
				outputFieldVal.Set(inputFieldVal.Convert(outputField.Type))
			} else {
				conversionIssues = append(conversionIssues,
					fmt.Sprintf("field '%s' type mismatch: %s -> %s",
						outputField.Name,
						inputField.Type.String(),
						outputField.Type.String()))
			}
		}
	}

	// Log conversion issues if any occurred
	if len(conversionIssues) > 0 {
		log.Debug().
			Str("struct_type", outputType.String()).
			Strs("conversion_issues", conversionIssues).
			Msg("Some fields could not be converted")
	}

	return outputVal.Interface().(T), nil
}

// IsNil safely checks if an interface is nil.
// It handles both direct nil values and nil pointers/interfaces.
// Works with pointer types, maps, arrays, channels, and slices.
//
// Parameters:
//   - i: The interface to check
//
// Returns:
//   - bool: true if the value is nil, false otherwise
func IsNil(i interface{}) bool {
	if i == nil {
		return true
	}

	switch reflect.TypeOf(i).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		return reflect.ValueOf(i).IsNil()
	}
	return false
}

// GetTypeString returns a string representation of the type of the input.
// If the input is nil, it returns "nil".
// Otherwise, it returns the type name using reflection.
//
// Parameters:
//   - i: The interface to get the type of
//
// Returns:
//   - string: The type name or "nil"
func GetTypeString(i interface{}) string {
	if i == nil {
		return "nil"
	}
	return reflect.TypeOf(i).String()
}
