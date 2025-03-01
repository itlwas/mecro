package action
import (
	"strings"
	"time"
	luar "layeh.com/gopher-luar"
	lua "github.com/yuin/gopher-lua"
	"github.com/zyedidia/micro/v2/internal/buffer"
	"github.com/zyedidia/micro/v2/internal/config"
	"github.com/zyedidia/micro/v2/internal/display"
	ulua "github.com/zyedidia/micro/v2/internal/lua"
	"github.com/zyedidia/micro/v2/internal/screen"
	"github.com/zyedidia/micro/v2/internal/util"
	"github.com/zyedidia/tcell/v2"
)
type BufAction interface{}
type BufKeyAction func(*BufPane) bool
type BufMouseAction func(*BufPane, *tcell.EventMouse) bool
var BufBindings *KeyTree
func BufKeyActionGeneral(a BufKeyAction) PaneKeyAction {
	return func(p Pane) bool {
		return a(p.(*BufPane))
	}
}
func BufMouseActionGeneral(a BufMouseAction) PaneMouseAction {
	return func(p Pane, me *tcell.EventMouse) bool {
		return a(p.(*BufPane), me)
	}
}
func init() {
	BufBindings = NewKeyTree()
}
func LuaAction(fn string, k Event) BufAction {
	luaFn := strings.Split(fn, ".")
	if len(luaFn) <= 1 {
		return nil
	}
	plName, plFn := luaFn[0], luaFn[1]
	pl := config.FindPlugin(plName)
	if pl == nil {
		return nil
	}
	var action BufAction
	switch k.(type) {
	case KeyEvent, KeySequenceEvent, RawEvent:
		action = BufKeyAction(func(h *BufPane) bool {
			val, err := pl.Call(plFn, luar.New(ulua.L, h))
			if err != nil {
				screen.TermMessage(err)
			}
			if v, ok := val.(lua.LBool); !ok {
				return false
			} else {
				return bool(v)
			}
		})
	case MouseEvent:
		action = BufMouseAction(func(h *BufPane, te *tcell.EventMouse) bool {
			val, err := pl.Call(plFn, luar.New(ulua.L, h), luar.New(ulua.L, te))
			if err != nil {
				screen.TermMessage(err)
			}
			if v, ok := val.(lua.LBool); !ok {
				return false
			} else {
				return bool(v)
			}
		})
	}
	return action
}
func BufMapEvent(k Event, action string) {
	config.Bindings["buffer"][k.Name()] = action
	var actionfns []BufAction
	var names []string
	var types []byte
	for i := 0; ; i++ {
		if action == "" {
			break
		}
		idx := strings.IndexAny(action, "&|,")
		a := action
		if idx >= 0 {
			a = action[:idx]
			types = append(types, action[idx])
			action = action[idx+1:]
		} else {
			types = append(types, ' ')
			action = ""
		}
		var afn BufAction
		if strings.HasPrefix(a, "command:") {
			a = strings.SplitN(a, ":", 2)[1]
			afn = CommandAction(a)
			names = append(names, "")
		} else if strings.HasPrefix(a, "command-edit:") {
			a = strings.SplitN(a, ":", 2)[1]
			afn = CommandEditAction(a)
			names = append(names, "")
		} else if strings.HasPrefix(a, "lua:") {
			a = strings.SplitN(a, ":", 2)[1]
			afn = LuaAction(a, k)
			if afn == nil {
				screen.TermMessage("Lua Error:", a, "does not exist")
				continue
			}
			split := strings.SplitN(a, ".", 2)
			if len(split) > 1 {
				a = strings.Title(split[0]) + strings.Title(split[1])
			} else {
				a = strings.Title(a)
			}
			names = append(names, a)
		} else if f, ok := BufKeyActions[a]; ok {
			afn = f
			names = append(names, a)
		} else if f, ok := BufMouseActions[a]; ok {
			afn = f
			names = append(names, a)
		} else {
			screen.TermMessage("Error in bindings: action", a, "does not exist")
			continue
		}
		actionfns = append(actionfns, afn)
	}
	bufAction := func(h *BufPane, te *tcell.EventMouse) bool {
		cursors := h.Buf.GetCursors()
		success := true
		for i, a := range actionfns {
			innerSuccess := true
			for j, c := range cursors {
				if c == nil {
					continue
				}
				h.Buf.SetCurCursor(c.Num)
				h.Cursor = c
				if i == 0 || (success && types[i-1] == '&') || (!success && types[i-1] == '|') || (types[i-1] == ',') {
					innerSuccess = innerSuccess && h.execAction(a, names[i], j, te)
				} else {
					break
				}
			}
			h = MainTab().CurPane()
			success = innerSuccess
		}
		return true
	}
	switch e := k.(type) {
	case KeyEvent, KeySequenceEvent, RawEvent:
		BufBindings.RegisterKeyBinding(e, BufKeyActionGeneral(func(h *BufPane) bool {
			return bufAction(h, nil)
		}))
	case MouseEvent:
		BufBindings.RegisterMouseBinding(e, BufMouseActionGeneral(bufAction))
	}
}
func BufUnmap(k Event) {
}
type BufPane struct {
	display.BWindow
	Buf *buffer.Buffer
	bindings *KeyTree
	Cursor *buffer.Cursor
	mousePressed map[MouseEvent]bool
	isOverwriteMode bool
	lastClickTime time.Time
	lastLoc       buffer.Loc
	lastCutTime time.Time
	freshClip bool
	doubleClick bool
	tripleClick bool
	multiWord bool
	splitID uint64
	tab     *Tab
	searchOrig buffer.Loc
	initialized bool
}
func newBufPane(buf *buffer.Buffer, win display.BWindow, tab *Tab) *BufPane {
	h := new(BufPane)
	h.Buf = buf
	h.BWindow = win
	h.tab = tab
	h.Cursor = h.Buf.GetActiveCursor()
	h.mousePressed = make(map[MouseEvent]bool)
	return h
}
func NewBufPane(buf *buffer.Buffer, win display.BWindow, tab *Tab) *BufPane {
	h := newBufPane(buf, win, tab)
	h.finishInitialize()
	return h
}
func NewBufPaneFromBuf(buf *buffer.Buffer, tab *Tab) *BufPane {
	w := display.NewBufWindow(0, 0, 0, 0, buf)
	h := newBufPane(buf, w, tab)
	return h
}
func (h *BufPane) finishInitialize() {
	h.initialRelocate()
	h.initialized = true
	config.RunPluginFn("onBufPaneOpen", luar.New(ulua.L, h))
}
func (h *BufPane) Resize(width, height int) {
	h.BWindow.Resize(width, height)
	if !h.initialized {
		h.finishInitialize()
	}
}
func (h *BufPane) SetTab(t *Tab) {
	h.tab = t
}
func (h *BufPane) Tab() *Tab {
	return h.tab
}
func (h *BufPane) ResizePane(size int) {
	n := h.tab.GetNode(h.splitID)
	n.ResizeSplit(size)
	h.tab.Resize()
}
func (h *BufPane) PluginCB(cb string) bool {
	b, err := config.RunPluginFnBool(h.Buf.Settings, cb, luar.New(ulua.L, h))
	if err != nil {
		screen.TermMessage(err)
	}
	return b
}
func (h *BufPane) PluginCBRune(cb string, r rune) bool {
	b, err := config.RunPluginFnBool(h.Buf.Settings, cb, luar.New(ulua.L, h), luar.New(ulua.L, string(r)))
	if err != nil {
		screen.TermMessage(err)
	}
	return b
}
func (h *BufPane) resetMouse() {
	for me := range h.mousePressed {
		delete(h.mousePressed, me)
	}
}
func (h *BufPane) OpenBuffer(b *buffer.Buffer) {
	h.Buf.Close()
	h.Buf = b
	h.BWindow.SetBuffer(b)
	h.Cursor = b.GetActiveCursor()
	h.Resize(h.GetView().Width, h.GetView().Height)
	h.initialRelocate()
	h.resetMouse()
	h.isOverwriteMode = false
	h.lastClickTime = time.Time{}
}
func (h *BufPane) GotoLoc(loc buffer.Loc) {
	sloc := h.SLocFromLoc(loc)
	d := h.Diff(h.SLocFromLoc(h.Cursor.Loc), sloc)
	h.Cursor.GotoLoc(loc)
	height := h.BufView().Height
	if util.Abs(d) >= height {
		v := h.GetView()
		v.StartLine = h.Scroll(sloc, -height/4)
		h.ScrollAdjust()
		v.StartCol = 0
	}
	h.Relocate()
}
func (h *BufPane) initialRelocate() {
	sloc := h.SLocFromLoc(h.Cursor.Loc)
	height := h.BufView().Height
	v := h.GetView()
	if h.Diff(display.SLoc{0, 0}, sloc) < height {
		v.StartLine = display.SLoc{0, 0}
	} else {
		v.StartLine = h.Scroll(sloc, -height/4)
		h.ScrollAdjust()
	}
	v.StartCol = 0
	h.Relocate()
}
func (h *BufPane) ID() uint64 {
	return h.splitID
}
func (h *BufPane) SetID(i uint64) {
	h.splitID = i
}
func (h *BufPane) Name() string {
	n := h.Buf.GetName()
	if h.Buf.Modified() {
		n += " +"
	}
	return n
}
func (h *BufPane) getReloadSetting() string {
	reloadSetting := h.Buf.Settings["reload"]
	return reloadSetting.(string)
}
func (h *BufPane) HandleEvent(event tcell.Event) {
	if h.Buf.ExternallyModified() && !h.Buf.ReloadDisabled {
		reload := h.getReloadSetting()
		if reload == "prompt" {
			InfoBar.YNPrompt("The file on disk has changed. Reload file? (y,n,esc)", func(yes, canceled bool) {
				if canceled {
					h.Buf.DisableReload()
				}
				if !yes || canceled {
					h.Buf.UpdateModTime()
				} else {
					h.Buf.ReOpen()
				}
			})
		} else if reload == "auto" {
			h.Buf.ReOpen()
		} else if reload == "disabled" {
			h.Buf.DisableReload()
		} else {
			InfoBar.Message("Invalid reload setting")
		}
	}
	switch e := event.(type) {
	case *tcell.EventRaw:
		re := RawEvent{
			esc: e.EscSeq(),
		}
		h.DoKeyEvent(re)
	case *tcell.EventPaste:
		h.paste(e.Text())
		h.Relocate()
	case *tcell.EventKey:
		ke := KeyEvent{
			code: e.Key(),
			mod:  metaToAlt(e.Modifiers()),
			r:    e.Rune(),
		}
		done := h.DoKeyEvent(ke)
		if !done && e.Key() == tcell.KeyRune {
			h.DoRuneInsert(e.Rune())
		}
	case *tcell.EventMouse:
		if e.Buttons() != tcell.ButtonNone {
			me := MouseEvent{
				btn:   e.Buttons(),
				mod:   metaToAlt(e.Modifiers()),
				state: MousePress,
			}
			isDrag := len(h.mousePressed) > 0
			if e.Buttons() & ^(tcell.WheelUp|tcell.WheelDown|tcell.WheelLeft|tcell.WheelRight) != tcell.ButtonNone {
				h.mousePressed[me] = true
			}
			if isDrag {
				me.state = MouseDrag
			}
			h.DoMouseEvent(me, e)
		} else {
			for me := range h.mousePressed {
				delete(h.mousePressed, me)
				me.state = MouseRelease
				h.DoMouseEvent(me, e)
			}
		}
	}
	h.Buf.MergeCursors()
	if h.IsActive() {
		c := h.Buf.GetActiveCursor()
		none := true
		for _, m := range h.Buf.Messages {
			if c.Y == m.Start.Y || c.Y == m.End.Y {
				InfoBar.GutterMessage(m.Msg)
				none = false
				break
			}
		}
		if none && InfoBar.HasGutter {
			InfoBar.ClearGutter()
		}
	}
	cursors := h.Buf.GetCursors()
	for _, c := range cursors {
		if c.NewTrailingWsY != c.Y && (!c.HasSelection() ||
			(c.NewTrailingWsY != c.CurSelection[0].Y && c.NewTrailingWsY != c.CurSelection[1].Y)) {
			c.NewTrailingWsY = -1
		}
	}
}
func (h *BufPane) Bindings() *KeyTree {
	if h.bindings != nil {
		return h.bindings
	}
	return BufBindings
}
func (h *BufPane) DoKeyEvent(e Event) bool {
	binds := h.Bindings()
	action, more := binds.NextEvent(e, nil)
	if action != nil && !more {
		action(h)
		binds.ResetEvents()
		return true
	} else if action == nil && !more {
		binds.ResetEvents()
	}
	return more
}
func (h *BufPane) execAction(action BufAction, name string, cursor int, te *tcell.EventMouse) bool {
	if name != "Autocomplete" && name != "CycleAutocompleteBack" {
		h.Buf.HasSuggestions = false
	}
	_, isMulti := MultiActions[name]
	if (!isMulti && cursor == 0) || isMulti {
		if h.PluginCB("pre" + name) {
			var success bool
			switch a := action.(type) {
			case BufKeyAction:
				success = a(h)
			case BufMouseAction:
				success = a(h, te)
			}
			success = success && h.PluginCB("on"+name)
			if isMulti {
				if recordingMacro {
					if name != "ToggleMacro" && name != "PlayMacro" {
						curmacro = append(curmacro, action)
					}
				}
			}
			return success
		}
	}
	return false
}
func (h *BufPane) completeAction(action string) {
	h.PluginCB("on" + action)
}
func (h *BufPane) HasKeyEvent(e Event) bool {
	return true
}
func (h *BufPane) DoMouseEvent(e MouseEvent, te *tcell.EventMouse) bool {
	binds := h.Bindings()
	action, _ := binds.NextEvent(e, te)
	if action != nil {
		action(h)
		binds.ResetEvents()
		return true
	}
	return false
}
func (h *BufPane) DoRuneInsert(r rune) {
	cursors := h.Buf.GetCursors()
	for _, c := range cursors {
		h.Buf.SetCurCursor(c.Num)
		h.Cursor = c
		if !h.PluginCBRune("preRune", r) {
			continue
		}
		if c.HasSelection() {
			c.DeleteSelection()
			c.ResetSelection()
		}
		if h.isOverwriteMode {
			next := c.Loc
			next.X++
			h.Buf.Replace(c.Loc, next, string(r))
		} else {
			h.Buf.Insert(c.Loc, string(r))
		}
		if recordingMacro {
			curmacro = append(curmacro, r)
		}
		h.Relocate()
		h.PluginCBRune("onRune", r)
	}
}
func (h *BufPane) VSplitIndex(buf *buffer.Buffer, right bool) *BufPane {
	e := NewBufPaneFromBuf(buf, h.tab)
	e.splitID = MainTab().GetNode(h.splitID).VSplit(right)
	MainTab().Panes = append(MainTab().Panes, e)
	MainTab().Resize()
	MainTab().SetActive(len(MainTab().Panes) - 1)
	return e
}
func (h *BufPane) HSplitIndex(buf *buffer.Buffer, bottom bool) *BufPane {
	e := NewBufPaneFromBuf(buf, h.tab)
	e.splitID = MainTab().GetNode(h.splitID).HSplit(bottom)
	MainTab().Panes = append(MainTab().Panes, e)
	MainTab().Resize()
	MainTab().SetActive(len(MainTab().Panes) - 1)
	return e
}
func (h *BufPane) VSplitBuf(buf *buffer.Buffer) *BufPane {
	return h.VSplitIndex(buf, h.Buf.Settings["splitright"].(bool))
}
func (h *BufPane) HSplitBuf(buf *buffer.Buffer) *BufPane {
	return h.HSplitIndex(buf, h.Buf.Settings["splitbottom"].(bool))
}
func (h *BufPane) Close() {
	h.Buf.Close()
}
func (h *BufPane) SetActive(b bool) {
	h.BWindow.SetActive(b)
	if b {
		c := h.Buf.GetActiveCursor()
		none := true
		for _, m := range h.Buf.Messages {
			if c.Y == m.Start.Y || c.Y == m.End.Y {
				InfoBar.GutterMessage(m.Msg)
				none = false
				break
			}
		}
		if none && InfoBar.HasGutter {
			InfoBar.ClearGutter()
		}
	}
}
var BufKeyActions = map[string]BufKeyAction{
	"CursorUp":                  (*BufPane).CursorUp,
	"CursorDown":                (*BufPane).CursorDown,
	"CursorPageUp":              (*BufPane).CursorPageUp,
	"CursorPageDown":            (*BufPane).CursorPageDown,
	"CursorLeft":                (*BufPane).CursorLeft,
	"CursorRight":               (*BufPane).CursorRight,
	"CursorStart":               (*BufPane).CursorStart,
	"CursorEnd":                 (*BufPane).CursorEnd,
	"SelectToStart":             (*BufPane).SelectToStart,
	"SelectToEnd":               (*BufPane).SelectToEnd,
	"SelectUp":                  (*BufPane).SelectUp,
	"SelectDown":                (*BufPane).SelectDown,
	"SelectLeft":                (*BufPane).SelectLeft,
	"SelectRight":               (*BufPane).SelectRight,
	"WordRight":                 (*BufPane).WordRight,
	"WordLeft":                  (*BufPane).WordLeft,
	"SelectWordRight":           (*BufPane).SelectWordRight,
	"SelectWordLeft":            (*BufPane).SelectWordLeft,
	"DeleteWordRight":           (*BufPane).DeleteWordRight,
	"DeleteWordLeft":            (*BufPane).DeleteWordLeft,
	"SelectLine":                (*BufPane).SelectLine,
	"SelectToStartOfLine":       (*BufPane).SelectToStartOfLine,
	"SelectToStartOfText":       (*BufPane).SelectToStartOfText,
	"SelectToStartOfTextToggle": (*BufPane).SelectToStartOfTextToggle,
	"SelectToEndOfLine":         (*BufPane).SelectToEndOfLine,
	"ParagraphPrevious":         (*BufPane).ParagraphPrevious,
	"ParagraphNext":             (*BufPane).ParagraphNext,
	"SelectParagraphPrevious":   (*BufPane).SelectParagraphPrevious,
	"SelectParagraphNext":       (*BufPane).SelectParagraphNext,
	"InsertNewline":             (*BufPane).InsertNewline,
	"Backspace":                 (*BufPane).Backspace,
	"Delete":                    (*BufPane).Delete,
	"InsertTab":                 (*BufPane).InsertTab,
	"Save":                      (*BufPane).Save,
	"SaveAll":                   (*BufPane).SaveAll,
	"SaveAs":                    (*BufPane).SaveAs,
	"Find":                      (*BufPane).Find,
	"FindLiteral":               (*BufPane).FindLiteral,
	"FindNext":                  (*BufPane).FindNext,
	"FindPrevious":              (*BufPane).FindPrevious,
	"DiffNext":                  (*BufPane).DiffNext,
	"DiffPrevious":              (*BufPane).DiffPrevious,
	"Center":                    (*BufPane).Center,
	"Undo":                      (*BufPane).Undo,
	"Redo":                      (*BufPane).Redo,
	"Copy":                      (*BufPane).Copy,
	"CopyLine":                  (*BufPane).CopyLine,
	"Cut":                       (*BufPane).Cut,
	"CutLine":                   (*BufPane).CutLine,
	"DuplicateLine":             (*BufPane).DuplicateLine,
	"DeleteLine":                (*BufPane).DeleteLine,
	"MoveLinesUp":               (*BufPane).MoveLinesUp,
	"MoveLinesDown":             (*BufPane).MoveLinesDown,
	"IndentSelection":           (*BufPane).IndentSelection,
	"OutdentSelection":          (*BufPane).OutdentSelection,
	"Autocomplete":              (*BufPane).Autocomplete,
	"CycleAutocompleteBack":     (*BufPane).CycleAutocompleteBack,
	"OutdentLine":               (*BufPane).OutdentLine,
	"IndentLine":                (*BufPane).IndentLine,
	"Paste":                     (*BufPane).Paste,
	"PastePrimary":              (*BufPane).PastePrimary,
	"SelectAll":                 (*BufPane).SelectAll,
	"OpenFile":                  (*BufPane).OpenFile,
	"Start":                     (*BufPane).Start,
	"End":                       (*BufPane).End,
	"PageUp":                    (*BufPane).PageUp,
	"PageDown":                  (*BufPane).PageDown,
	"SelectPageUp":              (*BufPane).SelectPageUp,
	"SelectPageDown":            (*BufPane).SelectPageDown,
	"HalfPageUp":                (*BufPane).HalfPageUp,
	"HalfPageDown":              (*BufPane).HalfPageDown,
	"StartOfText":               (*BufPane).StartOfText,
	"StartOfTextToggle":         (*BufPane).StartOfTextToggle,
	"StartOfLine":               (*BufPane).StartOfLine,
	"EndOfLine":                 (*BufPane).EndOfLine,
	"ToggleHelp":                (*BufPane).ToggleHelp,
	"ToggleKeyMenu":             (*BufPane).ToggleKeyMenu,
	"ToggleDiffGutter":          (*BufPane).ToggleDiffGutter,
	"ToggleRuler":               (*BufPane).ToggleRuler,
	"ToggleHighlightSearch":     (*BufPane).ToggleHighlightSearch,
	"UnhighlightSearch":         (*BufPane).UnhighlightSearch,
	"ClearStatus":               (*BufPane).ClearStatus,
	"ShellMode":                 (*BufPane).ShellMode,
	"CommandMode":               (*BufPane).CommandMode,
	"ToggleOverwriteMode":       (*BufPane).ToggleOverwriteMode,
	"Escape":                    (*BufPane).Escape,
	"Quit":                      (*BufPane).Quit,
	"QuitAll":                   (*BufPane).QuitAll,
	"ForceQuit":                 (*BufPane).ForceQuit,
	"AddTab":                    (*BufPane).AddTab,
	"PreviousTab":               (*BufPane).PreviousTab,
	"NextTab":                   (*BufPane).NextTab,
	"NextSplit":                 (*BufPane).NextSplit,
	"PreviousSplit":             (*BufPane).PreviousSplit,
	"Unsplit":                   (*BufPane).Unsplit,
	"VSplit":                    (*BufPane).VSplitAction,
	"HSplit":                    (*BufPane).HSplitAction,
	"ToggleMacro":               (*BufPane).ToggleMacro,
	"PlayMacro":                 (*BufPane).PlayMacro,
	"Suspend":                   (*BufPane).Suspend,
	"ScrollUp":                  (*BufPane).ScrollUpAction,
	"ScrollDown":                (*BufPane).ScrollDownAction,
	"SpawnMultiCursor":          (*BufPane).SpawnMultiCursor,
	"SpawnMultiCursorUp":        (*BufPane).SpawnMultiCursorUp,
	"SpawnMultiCursorDown":      (*BufPane).SpawnMultiCursorDown,
	"SpawnMultiCursorSelect":    (*BufPane).SpawnMultiCursorSelect,
	"RemoveMultiCursor":         (*BufPane).RemoveMultiCursor,
	"RemoveAllMultiCursors":     (*BufPane).RemoveAllMultiCursors,
	"SkipMultiCursor":           (*BufPane).SkipMultiCursor,
	"JumpToMatchingBrace":       (*BufPane).JumpToMatchingBrace,
	"JumpLine":                  (*BufPane).JumpLine,
	"Deselect":                  (*BufPane).Deselect,
	"ClearInfo":                 (*BufPane).ClearInfo,
	"None":                      (*BufPane).None,
	"InsertEnter": (*BufPane).InsertNewline,
}
var BufMouseActions = map[string]BufMouseAction{
	"MousePress":       (*BufPane).MousePress,
	"MouseDrag":        (*BufPane).MouseDrag,
	"MouseRelease":     (*BufPane).MouseRelease,
	"MouseMultiCursor": (*BufPane).MouseMultiCursor,
}
var MultiActions = map[string]bool{
	"CursorUp":                  true,
	"CursorDown":                true,
	"CursorPageUp":              true,
	"CursorPageDown":            true,
	"CursorLeft":                true,
	"CursorRight":               true,
	"CursorStart":               true,
	"CursorEnd":                 true,
	"SelectToStart":             true,
	"SelectToEnd":               true,
	"SelectUp":                  true,
	"SelectDown":                true,
	"SelectLeft":                true,
	"SelectRight":               true,
	"WordRight":                 true,
	"WordLeft":                  true,
	"SelectWordRight":           true,
	"SelectWordLeft":            true,
	"DeleteWordRight":           true,
	"DeleteWordLeft":            true,
	"SelectLine":                true,
	"SelectToStartOfLine":       true,
	"SelectToStartOfText":       true,
	"SelectToStartOfTextToggle": true,
	"SelectToEndOfLine":         true,
	"ParagraphPrevious":         true,
	"ParagraphNext":             true,
	"SelectParagraphPrevious":   true,
	"SelectParagraphNext":       true,
	"InsertNewline":             true,
	"Backspace":                 true,
	"Delete":                    true,
	"InsertTab":                 true,
	"FindNext":                  true,
	"FindPrevious":              true,
	"CopyLine":                  true,
	"Copy":                      true,
	"Cut":                       true,
	"CutLine":                   true,
	"DuplicateLine":             true,
	"DeleteLine":                true,
	"MoveLinesUp":               true,
	"MoveLinesDown":             true,
	"IndentSelection":           true,
	"OutdentSelection":          true,
	"OutdentLine":               true,
	"IndentLine":                true,
	"Paste":                     true,
	"PastePrimary":              true,
	"SelectPageUp":              true,
	"SelectPageDown":            true,
	"StartOfLine":               true,
	"StartOfText":               true,
	"StartOfTextToggle":         true,
	"EndOfLine":                 true,
	"JumpToMatchingBrace":       true,
}