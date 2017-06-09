package rproxy

import "net"

func IsTimeout(err error) bool {
	opErr, ok := err.(*net.OpError)
	return ok && opErr.Timeout()
}
