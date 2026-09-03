package availability

import "errors"

var ErrUnavailable = errors.New("a required dependency is unavailable")
