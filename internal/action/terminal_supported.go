// +build linux darwin dragonfly openbsd_amd64 freebsd

package action
import (
	shellquote "github.com/kballard/go-shellquote"
	"github.com/zyedidia/micro/v2/internal/shell"
)
const TermEmuSupported = true
func RunTermEmulator(h *BufPane, input string, wait bool, getOutput bool, callback func(out string, userargs []interface{}), userargs []interface{}) error {
	args, err := shellquote.Split(input)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return nil
	}
	t := new(shell.Terminal)
	err = t.Start(args, getOutput, wait, callback, userargs)
	if err != nil {
		return err
	}
	h.AddTab()
	id := MainTab().Panes[0].ID()
	v := h.GetView()
	tp, err := NewTermPane(v.X, v.Y, v.Width, v.Height, t, id, MainTab())
	if err != nil {
		return err
	}
	MainTab().Panes[0] = tp
	MainTab().SetActive(0)
	return nil
}