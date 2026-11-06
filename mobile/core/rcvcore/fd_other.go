//go:build !linux

package rcvcore

import "errors"

// The core only runs on Android; these stubs let its tests build elsewhere.

func tunName(int) (string, error) { return "", errors.New("unsupported") }
func dupFd(int) (int, error)      { return -1, errors.New("unsupported") }
func closeFd(int)                 {}
