package secrets

import "fmt"

type InvalidSecretError struct {
	Message string
}

func (e *InvalidSecretError) Error() string {
	return e.Message
}

func newInvalidSecretErrorf(msg string, a ...any) *InvalidSecretError {
	return &InvalidSecretError{
		Message: fmt.Sprintf(msg, a...),
	}
}
