package turtleware

import (
	"errors"
	"strings"
)

// ValidationWrapperError is a wrapper for indicating that the validation for a
// create or patch endpoint failed, via the containing errors.
type ValidationWrapperError struct {
	Errors []error
}

func (validationWrapperError ValidationWrapperError) Error() string {
	errorStrings := make([]string, len(validationWrapperError.Errors))

	for i, err := range validationWrapperError.Errors {
		errorStrings[i] = err.Error()
	}

	return strings.Join(errorStrings, ", ")
}

func (validationWrapperError ValidationWrapperError) As(target interface{}) bool {
	if w, ok := target.(*ValidationWrapperError); ok {
		*w = validationWrapperError
		return true
	}

	for _, err := range validationWrapperError.Errors {
		if errors.As(err, &target) {
			return true
		}
	}

	return false
}

func (validationWrapperError ValidationWrapperError) Unwrap() []error {
	return validationWrapperError.Errors
}
