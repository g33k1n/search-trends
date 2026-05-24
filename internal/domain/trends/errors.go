package trends

import "errors"

var (
	ErrEmptyQuery       = errors.New("query is empty")
	ErrQueryBlocked     = errors.New("query is blocked by stop-list")
	ErrEventOutOfWindow = errors.New("event timestamp is outside active window")
	ErrRateLimited      = errors.New("query exceeded per-bucket rate limit")
	ErrStopWordNotFound = errors.New("stop word not found")
)
