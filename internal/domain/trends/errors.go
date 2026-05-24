package trends

import "errors"

type ErrorCode int

const (
	CodeEmptyQuery       ErrorCode = 1001
	CodeQueryBlocked     ErrorCode = 1002
	CodeEventOutOfWindow ErrorCode = 1003
	CodeRateLimited      ErrorCode = 1004
	CodeStopWordNotFound ErrorCode = 1005
)

type DomainError struct {
	Code    ErrorCode
	Message string
}

func (e *DomainError) Error() string {
	return e.Message
}

func (e *DomainError) Is(target error) bool {
	t, ok := target.(*DomainError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

func CodeOf(err error) (ErrorCode, bool) {
	var de *DomainError
	if errors.As(err, &de) {
		return de.Code, true
	}
	return 0, false
}

var (
	ErrEmptyQuery       = &DomainError{Code: CodeEmptyQuery, Message: "query is empty"}
	ErrQueryBlocked     = &DomainError{Code: CodeQueryBlocked, Message: "query is blocked by stop-list"}
	ErrEventOutOfWindow = &DomainError{Code: CodeEventOutOfWindow, Message: "event timestamp is outside active window"}
	ErrRateLimited      = &DomainError{Code: CodeRateLimited, Message: "query exceeded per-bucket rate limit"}
	ErrStopWordNotFound = &DomainError{Code: CodeStopWordNotFound, Message: "stop word not found"}
)
