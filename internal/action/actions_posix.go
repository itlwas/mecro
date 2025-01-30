// +build linux darwin dragonfly solaris openbsd netbsd freebsd

package action
import (
	"syscall"
	"github.com/zyedidia/micro/v2/internal/screen"
)
func (*BufPane) Suspend() bool {
	screenb := screen.TempFini()
	pid := syscall.Getpid()
	err := syscall.Kill(pid, syscall.SIGSTOP)
	if err != nil {
		screen.TermMessage(err)
	}
	screen.TempStart(screenb)
	return false
}
