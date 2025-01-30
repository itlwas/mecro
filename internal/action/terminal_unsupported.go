// +build !linux,!darwin,!freebsd,!dragonfly,!openbsd_amd64

package action
import "errors"
const TermEmuSupported = false
func RunTermEmulator(input string, wait bool, getOutput bool, callback func(out string, userargs []interface{}), userargs []interface{}) error {
	return errors.New("Unsupported operating system")
}