package screen
import (
	"errors"
	"log"
	"os"
	"sync"
	"github.com/zyedidia/micro/v2/internal/config"
	"github.com/zyedidia/micro/v2/internal/util"
	"github.com/zyedidia/tcell/v2"
)
var Screen tcell.Screen
var Events chan (tcell.Event)
var RestartCallback func()
var lock sync.Mutex
var drawChan chan bool
func Lock() {
	lock.Lock()
}
func Unlock() {
	lock.Unlock()
}
func Redraw() {
	select {
	case drawChan <- true:
	default:
	}
}
func DrawChan() chan bool {
	return drawChan
}
type screenCell struct {
	x, y  int
	r     rune
	combc []rune
	style tcell.Style
}
var lastCursor screenCell
func ShowFakeCursor(x, y int) {
	r, combc, style, _ := Screen.GetContent(x, y)
	Screen.SetContent(lastCursor.x, lastCursor.y, lastCursor.r, lastCursor.combc, lastCursor.style)
	Screen.SetContent(x, y, r, combc, config.DefStyle.Reverse(true))
	lastCursor.x, lastCursor.y = x, y
	lastCursor.r = r
	lastCursor.combc = combc
	lastCursor.style = style
}
func UseFake() bool {
	return util.FakeCursor || config.GetGlobalOption("fakecursor").(bool)
}
func ShowFakeCursorMulti(x, y int) {
	r, _, _, _ := Screen.GetContent(x, y)
	Screen.SetContent(x, y, r, nil, config.DefStyle.Reverse(true))
}
func ShowCursor(x, y int) {
	if UseFake() {
		ShowFakeCursor(x, y)
	} else {
		Screen.ShowCursor(x, y)
	}
}
func SetContent(x, y int, mainc rune, combc []rune, style tcell.Style) {
	if !Screen.CanDisplay(mainc, true) {
		mainc = '�'
	}
	Screen.SetContent(x, y, mainc, combc, style)
	if UseFake() && lastCursor.x == x && lastCursor.y == y {
		lastCursor.r = mainc
		lastCursor.style = style
		lastCursor.combc = combc
	}
}
func TempFini() bool {
	screenWasNil := Screen == nil
	if !screenWasNil {
		Screen.Fini()
		Lock()
		Screen = nil
	}
	return screenWasNil
}
func TempStart(screenWasNil bool) {
	if !screenWasNil {
		Init()
		Unlock()
		if RestartCallback != nil {
			RestartCallback()
		}
	}
}
func Init() error {
	drawChan = make(chan bool, 8)
	var oldTerm string
	modifiedTerm := false
	setXterm := func() {
		oldTerm = os.Getenv("TERM")
		os.Setenv("TERM", "xterm-256color")
		modifiedTerm = true
	}
	if config.GetGlobalOption("xterm").(bool) {
		setXterm()
	}
	var err error
	Screen, err = tcell.NewScreen()
	if err != nil {
		log.Println("Warning: during screen initialization:", err)
		log.Println("Falling back to TERM=xterm-256color")
		setXterm()
		Screen, err = tcell.NewScreen()
		if err != nil {
			return err
		}
	}
	if err = Screen.Init(); err != nil {
		return err
	}
	Screen.SetPaste(config.GetGlobalOption("paste").(bool))
	if modifiedTerm {
		os.Setenv("TERM", oldTerm)
	}
	if config.GetGlobalOption("mouse").(bool) {
		Screen.EnableMouse()
	}
	return nil
}
func InitSimScreen() (tcell.SimulationScreen, error) {
	drawChan = make(chan bool, 8)
	var err error
	s := tcell.NewSimulationScreen("")
	if s == nil {
		return nil, errors.New("Failed to get a simulation screen")
	}
	if err = s.Init(); err != nil {
		return nil, err
	}
	s.SetSize(80, 24)
	Screen = s
	if config.GetGlobalOption("mouse").(bool) {
		Screen.EnableMouse()
	}
	return s, nil
}