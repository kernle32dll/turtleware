package turtleware_test

import (
	"github.com/kernle32dll/turtleware"
	"github.com/stretchr/testify/suite"

	"errors"
	"testing"
)

type ValidationWrapperErrorSuite struct {
	suite.Suite
}

func TestValidationWrapperErrorSuite(t *testing.T) {
	suite.Run(t, &ValidationWrapperErrorSuite{})
}

func (s *ValidationWrapperErrorSuite) Test_Error() {
	s.Run("Filled", func() {
		// given
		err := &turtleware.ValidationWrapperError{
			Errors: []error{
				errors.New("validation 1 error"),
				errors.New("validation 2 error"),
			},
		}

		// when
		res := err.Error()

		// then
		s.Equal("validation 1 error, validation 2 error", res)
	})
	s.Run("Empty", func() {
		// given
		err := &turtleware.ValidationWrapperError{
			Errors: nil,
		}

		// when
		res := err.Error()

		// then
		s.Equal("", res)
	})
}

func (s *ValidationWrapperErrorSuite) Test_Is() {
	// given
	chainedErr1 := errors.New("validation 1 error")
	chainedErr2 := errors.New("validation 2 error")
	err := &turtleware.ValidationWrapperError{
		Errors: []error{
			chainedErr1, chainedErr2,
		},
	}

	// when
	isChainedErr1 := errors.Is(err, chainedErr1)
	isChainedErr2 := errors.Is(err, chainedErr2)

	// then
	s.True(isChainedErr1)
	s.True(isChainedErr2)
}

type wrapped struct {
	msg string
	err error
}

func (e wrapped) Error() string { return e.msg }
func (e wrapped) Unwrap() error { return e.err }

func (s *ValidationWrapperErrorSuite) Test_As() {
	s.Run("Error_Itself", func() {
		// given
		chainedErr1 := errors.New("validation 1 error")
		chainedErr2 := errors.New("validation 2 error")
		err := &turtleware.ValidationWrapperError{
			Errors: []error{
				chainedErr1, chainedErr2,
			},
		}

		// when
		asValidationWrapperError := errors.As(err, &turtleware.ValidationWrapperError{})

		// then
		s.True(asValidationWrapperError)
	})
	s.Run("Wrapped", func() {
		// given
		chainedErr1 := errors.New("validation 1 error")
		wrappedErr := wrapped{err: chainedErr1}
		err := &turtleware.ValidationWrapperError{
			Errors: []error{
				wrappedErr,
			},
		}

		// when
		asValidationWrapperError := errors.As(err, &wrapped{})

		// then
		s.True(asValidationWrapperError)
	})
	s.Run("Unmatched", func() {
		// given
		err := &turtleware.ValidationWrapperError{}

		// when
		asValidationWrapperError := errors.As(err, &wrapped{})

		// then
		s.False(asValidationWrapperError)
	})
}
