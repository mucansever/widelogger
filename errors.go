package widelogger

import (
	"errors"
	"fmt"
)

var (
	ErrUninitializedContext = errors.New("widelogger: context not initialized with NewContext")
	ErrOddNumberOfArgs      = errors.New("widelogger: odd number of arguments in key-value pairs")
)

type ErrInvalidKey struct {
	Key any
}

func (e *ErrInvalidKey) Error() string {
	return fmt.Sprintf("widelogger: key must be string, got %T", e.Key)
}

var _ error = (*ErrInvalidKey)(nil)
