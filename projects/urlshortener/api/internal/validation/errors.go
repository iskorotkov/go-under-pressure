package validation

import "errors"

var (
	ErrEmptyURL            = errors.New("url is required")
	ErrInvalidURLFormat    = errors.New("invalid url format")
	ErrUnsafeProtocol      = errors.New("url protocol not allowed")
	ErrURLTooLong          = errors.New("url exceeds maximum length")
	ErrPrivateIPNotAllowed = errors.New("private ip addresses not allowed")
	ErrBatchTooLarge       = errors.New("batch size exceeds maximum")
	ErrEmptyBatch          = errors.New("urls is required")
)

type BatchValidationError struct {
	Errors []IndexedError
}

type IndexedError struct {
	Index int
	Err   error
}

func (e *BatchValidationError) Error() string {
	return "batch validation failed"
}
