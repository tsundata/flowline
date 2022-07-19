package net

import (
	"errors"
	"net"
	"reflect"
	"syscall"
)

// IPNetEqual checks if the two input IPNets are representing the same subnet.
// For example,
//	10.0.0.1/24 and 10.0.0.0/24 are the same subnet.
//	10.0.0.1/24 and 10.0.0.0/25 are not the same subnet.
func IPNetEqual(ipnet1, ipnet2 *net.IPNet) bool {
	if ipnet1 == nil || ipnet2 == nil {
		return false
	}
	if reflect.DeepEqual(ipnet1.Mask, ipnet2.Mask) && ipnet1.Contains(ipnet2.IP) && ipnet2.Contains(ipnet1.IP) {
		return true
	}
	return false
}

// Returns if the given err is "connection reset by peer" error.
func IsConnectionReset(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.ECONNRESET
	}
	return false
}

// Returns if the given err is "connection refused" error
func IsConnectionRefused(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.ECONNREFUSED
	}
	return false
}
