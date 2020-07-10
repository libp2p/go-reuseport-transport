package tcpreuse

import (
	"net"
	"os"
)

// reuseErrShouldRetry diagnoses whether to retry after a reuse error.
// if we failed to bind, we should retry. if bind worked and this is a
// real dial error (remote end didnt answer) then we should not retry.
func reuseErrShouldRetry(err error) bool {
	if err == nil {
		return false // hey, it worked! no need to retry.
	}

	// if it's a network timeout error, it's a legitimate failure.
	if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
		return false
	}

	e, ok := err.(*net.OpError)
	if !ok {
		return true
	}

	e1, ok := e.Err.(*os.PathError)
	if !ok {
		return true
	}

	switch e1.Err.Error() {
	case "address in use":
		return true
	case "connection refused":
		return false
	default:
		return true // optimistically default to retry.
	}
}
