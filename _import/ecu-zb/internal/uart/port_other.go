//go:build !linux

package uart

import (
	"errors"
	"os"
)

var errUnsupported = errors.New("ecu-zb only supports linux at runtime")

func OpenSerial(_ string) (*os.File, error) { return nil, errUnsupported }
func ConfigureRaw57600(_ int) error         { return errUnsupported }
