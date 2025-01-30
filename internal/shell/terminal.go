package shell
import (
	"bytes"
	"os/exec"
	"strconv"
	"github.com/zyedidia/micro/v2/internal/buffer"
	"github.com/zyedidia/micro/v2/internal/screen"
	"github.com/zyedidia/terminal"
)
type TermType int
type CallbackFunc func(string)
const (
	TTClose   = iota
	TTRunning
	TTDone
)
var CloseTerms chan bool
func init() {
	CloseTerms = make(chan bool)
}
type Terminal struct {
	State     terminal.State
	Term      *terminal.VT
	title     string
	Status    TermType
	Selection [2]buffer.Loc
	wait      bool
	getOutput bool
	output    *bytes.Buffer
	callback  CallbackFunc
}
func (t *Terminal) HasSelection() bool {
	return t.Selection[0] != t.Selection[1]
}
func (t *Terminal) Name() string {
	return t.title
}
func (t *Terminal) GetSelection(width int) string {
	start := t.Selection[0]
	end := t.Selection[1]
	if start.GreaterThan(end) {
		start, end = end, start
	}
	var ret string
	var l buffer.Loc
	for y := start.Y; y <= end.Y; y++ {
		for x := 0; x < width; x++ {
			l.X, l.Y = x, y
			if l.GreaterEqual(start) && l.LessThan(end) {
				c, _, _ := t.State.Cell(x, y)
				ret += string(c)
			}
		}
	}
	return ret
}
func (t *Terminal) Start(execCmd []string, getOutput bool, wait bool, callback func(out string, userargs []interface{}), userargs []interface{}) error {
	if len(execCmd) <= 0 {
		return nil
	}
	cmd := exec.Command(execCmd[0], execCmd[1:]...)
	t.output = nil
	if getOutput {
		t.output = bytes.NewBuffer([]byte{})
		cmd.Stdout = t.output
	}
	Term, _, err := terminal.Start(&t.State, cmd)
	if err != nil {
		return err
	}
	t.Term = Term
	t.getOutput = getOutput
	t.Status = TTRunning
	t.title = execCmd[0] + ":" + strconv.Itoa(cmd.Process.Pid)
	t.wait = wait
	t.callback = func(out string) {
		callback(out, userargs)
	}
	go func() {
		for {
			err := Term.Parse()
			if err != nil {
				Term.Write([]byte("Press enter to close"))
				screen.Redraw()
				break
			}
			screen.Redraw()
		}
		t.Stop()
	}()
	return nil
}
func (t *Terminal) Stop() {
	t.Term.File().Close()
	t.Term.Close()
	if t.wait {
		t.Status = TTDone
	} else {
		t.Close()
		CloseTerms <- true
	}
}
func (t *Terminal) Close() {
	t.Status = TTClose
	if t.getOutput {
		if t.callback != nil {
			Jobs <- JobFunction{
				Function: func(out string, args []interface{}) {
					t.callback(out)
				},
				Output: t.output.String(),
				Args:   nil,
			}
		}
	}
}
func (t *Terminal) WriteString(str string) {
	t.Term.File().WriteString(str)
}