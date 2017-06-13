package resp

import "net"

func IsNetTimeout(err error) bool {
	opErr, ok := err.(*net.OpError)
	return ok && opErr.Timeout()
}
