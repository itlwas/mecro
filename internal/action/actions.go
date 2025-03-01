package action
import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"
	shellquote "github.com/kballard/go-shellquote"
	"github.com/zyedidia/micro/v2/internal/buffer"
	"github.com/zyedidia/micro/v2/internal/clipboard"
	"github.com/zyedidia/micro/v2/internal/config"
	"github.com/zyedidia/micro/v2/internal/display"
	"github.com/zyedidia/micro/v2/internal/screen"
	"github.com/zyedidia/micro/v2/internal/shell"
	"github.com/zyedidia/micro/v2/internal/util"
	"github.com/zyedidia/tcell/v2"
)
func (h *BufPane) ScrollUp(n int) {
	v := h.GetView()
	v.StartLine = h.Scroll(v.StartLine, -n)
	h.SetView(v)
}
func (h *BufPane) ScrollDown(n int) {
	v := h.GetView()
	v.StartLine = h.Scroll(v.StartLine, n)
	h.SetView(v)
}
func (h *BufPane) ScrollAdjust() {
	v := h.GetView()
	end := h.SLocFromLoc(h.Buf.End())
	if h.Diff(v.StartLine, end) < h.BufView().Height-1 {
		v.StartLine = h.Scroll(end, -h.BufView().Height+1)
	}
	h.SetView(v)
}
func (h *BufPane) MousePress(e *tcell.EventMouse) bool {
	b := h.Buf
	mx, my := e.Position()
	if my >= h.BufView().Y+h.BufView().Height {
		return false
	}
	mouseLoc := h.LocFromVisual(buffer.Loc{mx, my})
	h.Cursor.Loc = mouseLoc
	if b.NumCursors() > 1 {
		b.ClearCursors()
		h.Relocate()
		h.Cursor = h.Buf.GetActiveCursor()
		h.Cursor.Loc = mouseLoc
	}
	if time.Since(h.lastClickTime)/time.Millisecond < config.DoubleClickThreshold && (mouseLoc.X == h.lastLoc.X && mouseLoc.Y == h.lastLoc.Y) {
		if h.doubleClick {
			h.lastClickTime = time.Now()
			h.tripleClick = true
			h.doubleClick = false
			h.Cursor.SelectLine()
			h.Cursor.CopySelection(clipboard.PrimaryReg)
		} else {
			h.lastClickTime = time.Now()
			h.doubleClick = true
			h.tripleClick = false
			h.Cursor.SelectWord()
			h.Cursor.CopySelection(clipboard.PrimaryReg)
		}
	} else {
		h.doubleClick = false
		h.tripleClick = false
		h.lastClickTime = time.Now()
		h.Cursor.OrigSelection[0] = h.Cursor.Loc
		h.Cursor.CurSelection[0] = h.Cursor.Loc
		h.Cursor.CurSelection[1] = h.Cursor.Loc
	}
	h.Cursor.StoreVisualX()
	h.lastLoc = mouseLoc
	h.Relocate()
	return true
}
func (h *BufPane) MouseDrag(e *tcell.EventMouse) bool {
	mx, my := e.Position()
	if my >= h.BufView().Y+h.BufView().Height {
		return false
	}
	h.Cursor.Loc = h.LocFromVisual(buffer.Loc{mx, my})
	if h.tripleClick {
		h.Cursor.AddLineToSelection()
	} else if h.doubleClick {
		h.Cursor.AddWordToSelection()
	} else {
		h.Cursor.SetSelectionEnd(h.Cursor.Loc)
	}
	h.Cursor.StoreVisualX()
	h.Relocate()
	return true
}
func (h *BufPane) MouseRelease(e *tcell.EventMouse) bool {
	if h.Cursor.HasSelection() {
		h.Cursor.CopySelection(clipboard.PrimaryReg)
	}
	return true
}
func (h *BufPane) ScrollUpAction() bool {
	h.ScrollUp(util.IntOpt(h.Buf.Settings["scrollspeed"]))
	return true
}
func (h *BufPane) ScrollDownAction() bool {
	h.ScrollDown(util.IntOpt(h.Buf.Settings["scrollspeed"]))
	return true
}
func (h *BufPane) Center() bool {
	v := h.GetView()
	v.StartLine = h.Scroll(h.SLocFromLoc(h.Cursor.Loc), -h.BufView().Height/2)
	h.SetView(v)
	h.ScrollAdjust()
	return true
}
func (h *BufPane) MoveCursorUp(n int) {
	if !h.Buf.Settings["softwrap"].(bool) {
		h.Cursor.UpN(n)
	} else {
		vloc := h.VLocFromLoc(h.Cursor.Loc)
		sloc := h.Scroll(vloc.SLoc, -n)
		if sloc == vloc.SLoc {
			h.Cursor.Loc = h.Buf.Start()
			h.Cursor.LastVisualX = 0
		} else {
			vloc.SLoc = sloc
			vloc.VisualX = h.Cursor.LastVisualX
			h.Cursor.Loc = h.LocFromVLoc(vloc)
		}
	}
}
func (h *BufPane) MoveCursorDown(n int) {
	if !h.Buf.Settings["softwrap"].(bool) {
		h.Cursor.DownN(n)
	} else {
		vloc := h.VLocFromLoc(h.Cursor.Loc)
		sloc := h.Scroll(vloc.SLoc, n)
		if sloc == vloc.SLoc {
			h.Cursor.Loc = h.Buf.End()
			vloc = h.VLocFromLoc(h.Cursor.Loc)
			h.Cursor.LastVisualX = vloc.VisualX
		} else {
			vloc.SLoc = sloc
			vloc.VisualX = h.Cursor.LastVisualX
			h.Cursor.Loc = h.LocFromVLoc(vloc)
		}
	}
}
func (h *BufPane) CursorUp() bool {
	h.Cursor.Deselect(true)
	h.MoveCursorUp(1)
	h.Relocate()
	return true
}
func (h *BufPane) CursorDown() bool {
	h.Cursor.Deselect(true)
	h.MoveCursorDown(1)
	h.Relocate()
	return true
}
func (h *BufPane) CursorLeft() bool {
	if h.Cursor.HasSelection() {
		h.Cursor.Deselect(true)
	} else {
		tabstospaces := h.Buf.Settings["tabstospaces"].(bool)
		tabmovement := h.Buf.Settings["tabmovement"].(bool)
		if tabstospaces && tabmovement {
			tabsize := int(h.Buf.Settings["tabsize"].(float64))
			line := h.Buf.LineBytes(h.Cursor.Y)
			if h.Cursor.X-tabsize >= 0 && util.IsSpaces(line[h.Cursor.X-tabsize:h.Cursor.X]) && util.IsBytesWhitespace(line[0:h.Cursor.X-tabsize]) {
				for i := 0; i < tabsize; i++ {
					h.Cursor.Left()
				}
			} else {
				h.Cursor.Left()
			}
		} else {
			h.Cursor.Left()
		}
	}
	h.Relocate()
	return true
}
func (h *BufPane) CursorRight() bool {
	if h.Cursor.HasSelection() {
		h.Cursor.Deselect(false)
		h.Cursor.Right()
	} else {
		tabstospaces := h.Buf.Settings["tabstospaces"].(bool)
		tabmovement := h.Buf.Settings["tabmovement"].(bool)
		if tabstospaces && tabmovement {
			tabsize := int(h.Buf.Settings["tabsize"].(float64))
			line := h.Buf.LineBytes(h.Cursor.Y)
			if h.Cursor.X+tabsize < util.CharacterCount(line) && util.IsSpaces(line[h.Cursor.X:h.Cursor.X+tabsize]) && util.IsBytesWhitespace(line[0:h.Cursor.X]) {
				for i := 0; i < tabsize; i++ {
					h.Cursor.Right()
				}
			} else {
				h.Cursor.Right()
			}
		} else {
			h.Cursor.Right()
		}
	}
	h.Relocate()
	return true
}
func (h *BufPane) WordRight() bool {
	h.Cursor.Deselect(false)
	h.Cursor.WordRight()
	h.Relocate()
	return true
}
func (h *BufPane) WordLeft() bool {
	h.Cursor.Deselect(true)
	h.Cursor.WordLeft()
	h.Relocate()
	return true
}
func (h *BufPane) SelectUp() bool {
	if !h.Cursor.HasSelection() {
		h.Cursor.OrigSelection[0] = h.Cursor.Loc
	}
	h.MoveCursorUp(1)
	h.Cursor.SelectTo(h.Cursor.Loc)
	h.Relocate()
	return true
}
func (h *BufPane) SelectDown() bool {
	if !h.Cursor.HasSelection() {
		h.Cursor.OrigSelection[0] = h.Cursor.Loc
	}
	h.MoveCursorDown(1)
	h.Cursor.SelectTo(h.Cursor.Loc)
	h.Relocate()
	return true
}
func (h *BufPane) SelectLeft() bool {
	loc := h.Cursor.Loc
	count := h.Buf.End()
	if loc.GreaterThan(count) {
		loc = count
	}
	if !h.Cursor.HasSelection() {
		h.Cursor.OrigSelection[0] = loc
	}
	h.Cursor.Left()
	h.Cursor.SelectTo(h.Cursor.Loc)
	h.Relocate()
	return true
}
func (h *BufPane) SelectRight() bool {
	loc := h.Cursor.Loc
	count := h.Buf.End()
	if loc.GreaterThan(count) {
		loc = count
	}
	if !h.Cursor.HasSelection() {
		h.Cursor.OrigSelection[0] = loc
	}
	h.Cursor.Right()
	h.Cursor.SelectTo(h.Cursor.Loc)
	h.Relocate()
	return true
}
func (h *BufPane) SelectWordRight() bool {
	if !h.Cursor.HasSelection() {
		h.Cursor.OrigSelection[0] = h.Cursor.Loc
	}
	h.Cursor.WordRight()
	h.Cursor.SelectTo(h.Cursor.Loc)
	h.Relocate()
	return true
}
func (h *BufPane) SelectWordLeft() bool {
	if !h.Cursor.HasSelection() {
		h.Cursor.OrigSelection[0] = h.Cursor.Loc
	}
	h.Cursor.WordLeft()
	h.Cursor.SelectTo(h.Cursor.Loc)
	h.Relocate()
	return true
}
func (h *BufPane) StartOfText() bool {
	h.Cursor.Deselect(true)
	h.Cursor.StartOfText()
	h.Relocate()
	return true
}
func (h *BufPane) StartOfTextToggle() bool {
	h.Cursor.Deselect(true)
	if h.Cursor.IsStartOfText() {
		h.Cursor.Start()
	} else {
		h.Cursor.StartOfText()
	}
	h.Relocate()
	return true
}
func (h *BufPane) StartOfLine() bool {
	h.Cursor.Deselect(true)
	h.Cursor.Start()
	h.Relocate()
	return true
}
func (h *BufPane) EndOfLine() bool {
	h.Cursor.Deselect(true)
	h.Cursor.End()
	h.Relocate()
	return true
}
func (h *BufPane) SelectLine() bool {
	h.Cursor.SelectLine()
	h.Relocate()
	return true
}
func (h *BufPane) SelectToStartOfText() bool {
	if !h.Cursor.HasSelection() {
		h.Cursor.OrigSelection[0] = h.Cursor.Loc
	}
	h.Cursor.StartOfText()
	h.Cursor.SelectTo(h.Cursor.Loc)
	h.Relocate()
	return true
}
func (h *BufPane) SelectToStartOfTextToggle() bool {
	if !h.Cursor.HasSelection() {
		h.Cursor.OrigSelection[0] = h.Cursor.Loc
	}
	if h.Cursor.IsStartOfText() {
		h.Cursor.Start()
	} else {
		h.Cursor.StartOfText()
	}
	h.Cursor.SelectTo(h.Cursor.Loc)
	h.Relocate()
	return true
}
func (h *BufPane) SelectToStartOfLine() bool {
	if !h.Cursor.HasSelection() {
		h.Cursor.OrigSelection[0] = h.Cursor.Loc
	}
	h.Cursor.Start()
	h.Cursor.SelectTo(h.Cursor.Loc)
	h.Relocate()
	return true
}
func (h *BufPane) SelectToEndOfLine() bool {
	if !h.Cursor.HasSelection() {
		h.Cursor.OrigSelection[0] = h.Cursor.Loc
	}
	h.Cursor.End()
	h.Cursor.SelectTo(h.Cursor.Loc)
	h.Relocate()
	return true
}
func (h *BufPane) ParagraphPrevious() bool {
	var line int
	for line = h.Cursor.Y; line > 0; line-- {
		if len(h.Buf.LineBytes(line)) == 0 && line != h.Cursor.Y {
			h.Cursor.X = 0
			h.Cursor.Y = line
			break
		}
	}
	if line == 0 {
		h.Cursor.Loc = h.Buf.Start()
	}
	h.Relocate()
	return true
}
func (h *BufPane) ParagraphNext() bool {
	var line int
	for line = h.Cursor.Y; line < h.Buf.LinesNum(); line++ {
		if len(h.Buf.LineBytes(line)) == 0 && line != h.Cursor.Y {
			h.Cursor.X = 0
			h.Cursor.Y = line
			break
		}
	}
	if line == h.Buf.LinesNum() {
		h.Cursor.Loc = h.Buf.End()
	}
	h.Relocate()
	return true
}
func (h *BufPane) SelectParagraphPrevious() bool {
	var line int
	if !h.Cursor.HasSelection() {
		h.Cursor.OrigSelection[0] = h.Cursor.Loc
	}
	for line = h.Cursor.Y; line > 0; line-- {
		if len(h.Buf.LineBytes(line)) == 0 && line != h.Cursor.Y {
			h.Cursor.X = 0
			h.Cursor.Y = line
			break
		}
	}
	if line == 0 {
		h.Cursor.Loc = h.Buf.Start()
	}
	h.Cursor.SelectTo(h.Cursor.Loc)
	return true
}
func (h *BufPane) SelectParagraphNext() bool {
	var line int
	if !h.Cursor.HasSelection() {
		h.Cursor.OrigSelection[0] = h.Cursor.Loc
	}
	for line = h.Cursor.Y; line < h.Buf.LinesNum(); line++ {
		if len(h.Buf.LineBytes(line)) == 0 && line != h.Cursor.Y {
			h.Cursor.X = 0
			h.Cursor.Y = line
			break
		}
	}
	if line == h.Buf.LinesNum() {
		h.Cursor.Loc = h.Buf.End()
	}
	h.Cursor.SelectTo(h.Cursor.Loc)
	return true
}
func (h *BufPane) Retab() bool {
	h.Buf.Retab()
	h.Relocate()
	return true
}
func (h *BufPane) CursorStart() bool {
	h.Cursor.Deselect(true)
	h.Cursor.X = 0
	h.Cursor.Y = 0
	h.Cursor.StoreVisualX()
	h.Relocate()
	return true
}
func (h *BufPane) CursorEnd() bool {
	h.Cursor.Deselect(true)
	h.Cursor.Loc = h.Buf.End()
	h.Cursor.StoreVisualX()
	h.Relocate()
	return true
}
func (h *BufPane) SelectToStart() bool {
	if !h.Cursor.HasSelection() {
		h.Cursor.OrigSelection[0] = h.Cursor.Loc
	}
	h.CursorStart()
	h.Cursor.SelectTo(h.Buf.Start())
	h.Relocate()
	return true
}
func (h *BufPane) SelectToEnd() bool {
	if !h.Cursor.HasSelection() {
		h.Cursor.OrigSelection[0] = h.Cursor.Loc
	}
	h.CursorEnd()
	h.Cursor.SelectTo(h.Buf.End())
	h.Relocate()
	return true
}
func (h *BufPane) InsertNewline() bool {
	if h.Cursor.HasSelection() {
		h.Cursor.DeleteSelection()
		h.Cursor.ResetSelection()
	}
	ws := util.GetLeadingWhitespace(h.Buf.LineBytes(h.Cursor.Y))
	cx := h.Cursor.X
	h.Buf.Insert(h.Cursor.Loc, "\n")
	if h.Buf.Settings["autoindent"].(bool) {
		if cx < len(ws) {
			ws = ws[0:cx]
		}
		h.Buf.Insert(h.Cursor.Loc, string(ws))
		if util.IsSpacesOrTabs(h.Buf.LineBytes(h.Cursor.Y-1)) && !h.Buf.Settings["keepautoindent"].(bool) {
			line := h.Buf.LineBytes(h.Cursor.Y - 1)
			h.Buf.Remove(buffer.Loc{X: 0, Y: h.Cursor.Y - 1}, buffer.Loc{X: util.CharacterCount(line), Y: h.Cursor.Y - 1})
		}
	}
	h.Cursor.LastVisualX = h.Cursor.GetVisualX()
	h.Relocate()
	return true
}
func (h *BufPane) Backspace() bool {
	if h.Cursor.HasSelection() {
		h.Cursor.DeleteSelection()
		h.Cursor.ResetSelection()
	} else if h.Cursor.Loc.GreaterThan(h.Buf.Start()) {
		lineStart := util.SliceStart(h.Buf.LineBytes(h.Cursor.Y), h.Cursor.X)
		tabSize := int(h.Buf.Settings["tabsize"].(float64))
		if h.Buf.Settings["tabstospaces"].(bool) && util.IsSpaces(lineStart) && len(lineStart) != 0 && util.CharacterCount(lineStart)%tabSize == 0 {
			loc := h.Cursor.Loc
			h.Buf.Remove(loc.Move(-tabSize, h.Buf), loc)
		} else {
			loc := h.Cursor.Loc
			h.Buf.Remove(loc.Move(-1, h.Buf), loc)
		}
	}
	h.Cursor.LastVisualX = h.Cursor.GetVisualX()
	h.Relocate()
	return true
}
func (h *BufPane) DeleteWordRight() bool {
	h.SelectWordRight()
	if h.Cursor.HasSelection() {
		h.Cursor.DeleteSelection()
		h.Cursor.ResetSelection()
	}
	h.Relocate()
	return true
}
func (h *BufPane) DeleteWordLeft() bool {
	h.SelectWordLeft()
	if h.Cursor.HasSelection() {
		h.Cursor.DeleteSelection()
		h.Cursor.ResetSelection()
	}
	h.Relocate()
	return true
}
func (h *BufPane) Delete() bool {
	if h.Cursor.HasSelection() {
		h.Cursor.DeleteSelection()
		h.Cursor.ResetSelection()
	} else {
		loc := h.Cursor.Loc
		if loc.LessThan(h.Buf.End()) {
			h.Buf.Remove(loc, loc.Move(1, h.Buf))
		}
	}
	h.Relocate()
	return true
}
func (h *BufPane) IndentSelection() bool {
	if h.Cursor.HasSelection() {
		start := h.Cursor.CurSelection[0]
		end := h.Cursor.CurSelection[1]
		if end.Y < start.Y {
			start, end = end, start
			h.Cursor.SetSelectionStart(start)
			h.Cursor.SetSelectionEnd(end)
		}
		startY := start.Y
		endY := end.Move(-1, h.Buf).Y
		endX := end.Move(-1, h.Buf).X
		tabsize := int(h.Buf.Settings["tabsize"].(float64))
		indentsize := len(h.Buf.IndentString(tabsize))
		for y := startY; y <= endY; y++ {
			if len(h.Buf.LineBytes(y)) > 0 {
				h.Buf.Insert(buffer.Loc{X: 0, Y: y}, h.Buf.IndentString(tabsize))
				if y == startY && start.X > 0 {
					h.Cursor.SetSelectionStart(start.Move(indentsize, h.Buf))
				}
				if y == endY {
					h.Cursor.SetSelectionEnd(buffer.Loc{X: endX + indentsize + 1, Y: endY})
				}
			}
		}
		h.Buf.RelocateCursors()
		h.Relocate()
		return true
	}
	return false
}
func (h *BufPane) IndentLine() bool {
	if h.Cursor.HasSelection() {
		return false
	}
	tabsize := int(h.Buf.Settings["tabsize"].(float64))
	indentstr := h.Buf.IndentString(tabsize)
	h.Buf.Insert(buffer.Loc{X: 0, Y: h.Cursor.Y}, indentstr)
	h.Buf.RelocateCursors()
	h.Relocate()
	return true
}
func (h *BufPane) OutdentLine() bool {
	if h.Cursor.HasSelection() {
		return false
	}
	for x := 0; x < len(h.Buf.IndentString(util.IntOpt(h.Buf.Settings["tabsize"]))); x++ {
		if len(util.GetLeadingWhitespace(h.Buf.LineBytes(h.Cursor.Y))) == 0 {
			break
		}
		h.Buf.Remove(buffer.Loc{X: 0, Y: h.Cursor.Y}, buffer.Loc{X: 1, Y: h.Cursor.Y})
	}
	h.Buf.RelocateCursors()
	h.Relocate()
	return true
}
func (h *BufPane) OutdentSelection() bool {
	if h.Cursor.HasSelection() {
		start := h.Cursor.CurSelection[0]
		end := h.Cursor.CurSelection[1]
		if end.Y < start.Y {
			start, end = end, start
			h.Cursor.SetSelectionStart(start)
			h.Cursor.SetSelectionEnd(end)
		}
		startY := start.Y
		endY := end.Move(-1, h.Buf).Y
		for y := startY; y <= endY; y++ {
			for x := 0; x < len(h.Buf.IndentString(util.IntOpt(h.Buf.Settings["tabsize"]))); x++ {
				if len(util.GetLeadingWhitespace(h.Buf.LineBytes(y))) == 0 {
					break
				}
				h.Buf.Remove(buffer.Loc{X: 0, Y: y}, buffer.Loc{X: 1, Y: y})
			}
		}
		h.Buf.RelocateCursors()
		h.Relocate()
		return true
	}
	return false
}
func (h *BufPane) Autocomplete() bool {
	b := h.Buf
	if h.Cursor.HasSelection() {
		return false
	}
	if h.Cursor.X == 0 {
		return false
	}
	r := h.Cursor.RuneUnder(h.Cursor.X)
	prev := h.Cursor.RuneUnder(h.Cursor.X - 1)
	if !util.IsAutocomplete(prev) || !util.IsNonAlphaNumeric(r) {
		return false
	}
	if b.HasSuggestions {
		b.CycleAutocomplete(true)
		return true
	}
	return b.Autocomplete(buffer.BufferComplete)
}
func (h *BufPane) CycleAutocompleteBack() bool {
	if h.Cursor.HasSelection() {
		return false
	}
	if h.Buf.HasSuggestions {
		h.Buf.CycleAutocomplete(false)
		return true
	}
	return false
}
func (h *BufPane) InsertTab() bool {
	b := h.Buf
	indent := b.IndentString(util.IntOpt(b.Settings["tabsize"]))
	tabBytes := len(indent)
	bytesUntilIndent := tabBytes - (h.Cursor.GetVisualX() % tabBytes)
	b.Insert(h.Cursor.Loc, indent[:bytesUntilIndent])
	h.Relocate()
	return true
}
func (h *BufPane) SaveAll() bool {
	for _, b := range buffer.OpenBuffers {
		b.Save()
	}
	return true
}
func (h *BufPane) SaveCB(action string, callback func()) bool {
	if h.Buf.Path == "" {
		h.SaveAsCB(action, callback)
	} else {
		noPrompt := h.saveBufToFile(h.Buf.Path, action, callback)
		if noPrompt {
			return true
		}
	}
	return false
}
func (h *BufPane) Save() bool {
	return h.SaveCB("Save", nil)
}
func (h *BufPane) SaveAsCB(action string, callback func()) bool {
	InfoBar.Prompt("Filename: ", "", "Save", nil, func(resp string, canceled bool) {
		if !canceled {
			args, err := shellquote.Split(resp)
			if err != nil {
				InfoBar.Error("Error parsing arguments: ", err)
				return
			}
			if len(args) == 0 {
				InfoBar.Error("No filename given")
				return
			}
			filename := strings.Join(args, " ")
			fileinfo, err := os.Stat(filename)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
					noPrompt := h.saveBufToFile(filename, action, callback)
					if noPrompt {
						h.completeAction(action)
						return
					}
				}
			} else {
				InfoBar.YNPrompt(
					fmt.Sprintf("The file %s already exists in the directory, would you like to overwrite? Y/n", fileinfo.Name()),
					func(yes, canceled bool) {
						if yes && !canceled {
							noPrompt := h.saveBufToFile(filename, action, callback)
							if noPrompt {
								h.completeAction(action)
							}
						}
					},
				)
			}
		}
	})
	return false
}
func (h *BufPane) SaveAs() bool {
	return h.SaveAsCB("SaveAs", nil)
}
func (h *BufPane) saveBufToFile(filename string, action string, callback func()) bool {
	err := h.Buf.SaveAs(filename)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			saveWithSudo := func() {
				err = h.Buf.SaveAsWithSudo(filename)
				if err != nil {
					InfoBar.Error(err)
				} else {
					h.Buf.Path = filename
					h.Buf.SetName(filename)
					InfoBar.Message("Saved " + filename)
					if callback != nil {
						callback()
					}
				}
			}
			if h.Buf.Settings["autosu"].(bool) {
				saveWithSudo()
			} else {
				InfoBar.YNPrompt(
					fmt.Sprintf("Permission denied. Do you want to save this file using %s? (y,n)", config.GlobalSettings["sucmd"].(string)),
					func(yes, canceled bool) {
						if yes && !canceled {
							saveWithSudo()
							h.completeAction(action)
						}
					},
				)
				return false
			}
		} else {
			InfoBar.Error(err)
		}
	} else {
		h.Buf.Path = filename
		h.Buf.SetName(filename)
		InfoBar.Message("Saved " + filename)
		if callback != nil {
			callback()
		}
	}
	return true
}
func (h *BufPane) Find() bool {
	return h.find(true)
}
func (h *BufPane) FindLiteral() bool {
	return h.find(false)
}
func (h *BufPane) Search(str string, useRegex bool, searchDown bool) error {
	match, found, err := h.Buf.FindNext(str, h.Buf.Start(), h.Buf.End(), h.Cursor.Loc, searchDown, useRegex)
	if err != nil {
		return err
	}
	if found {
		h.Cursor.SetSelectionStart(match[0])
		h.Cursor.SetSelectionEnd(match[1])
		h.Cursor.OrigSelection[0] = h.Cursor.CurSelection[0]
		h.Cursor.OrigSelection[1] = h.Cursor.CurSelection[1]
		h.GotoLoc(h.Cursor.CurSelection[1])
		h.Buf.LastSearch = str
		h.Buf.LastSearchRegex = useRegex
		h.Buf.HighlightSearch = h.Buf.Settings["hlsearch"].(bool)
	} else {
		h.Cursor.ResetSelection()
	}
	return nil
}
func (h *BufPane) find(useRegex bool) bool {
    h.searchOrig = h.Cursor.Loc
    prompt := "Find: "
    if useRegex {
        prompt = "Find (regex): "
    }
    var eventCallback func(resp string)
    if h.Buf.Settings["incsearch"].(bool) {
        eventCallback = func(resp string) {
            match, found, _ := h.Buf.FindNext(resp, h.Buf.Start(), h.Buf.End(), h.searchOrig, true, useRegex)
            if found {
                h.Cursor.SetSelectionStart(match[0])
                h.Cursor.SetSelectionEnd(match[1])
                h.Cursor.OrigSelection[0] = h.Cursor.CurSelection[0]
                h.Cursor.OrigSelection[1] = h.Cursor.CurSelection[1]
                h.GotoLoc(match[1])
                total := 0
                start := h.Buf.Start()
                for {
                    m, f, _ := h.Buf.FindNext(resp, start, h.Buf.End(), start, true, useRegex)
                    if !f {
                        break
                    }
                    total++
                    start = m[1]
                }
                InfoBar.Message(fmt.Sprintf("Found %d matches", total))
            } else {
                h.GotoLoc(h.searchOrig)
                h.Cursor.ResetSelection()
                InfoBar.Message("No matches")
            }
        }
    }
    findCallback := func(resp string, canceled bool) {
        if !canceled {
            match, found, err := h.Buf.FindNext(resp, h.Buf.Start(), h.Buf.End(), h.searchOrig, true, useRegex)
            if err != nil {
                InfoBar.Error(err)
            }
            if found {
                h.Cursor.SetSelectionStart(match[0])
                h.Cursor.SetSelectionEnd(match[1])
                h.Cursor.OrigSelection[0] = h.Cursor.CurSelection[0]
                h.Cursor.OrigSelection[1] = h.Cursor.CurSelection[1]
                h.GotoLoc(h.Cursor.CurSelection[1])
                h.Buf.LastSearch = resp
                h.Buf.LastSearchRegex = useRegex
                h.Buf.HighlightSearch = h.Buf.Settings["hlsearch"].(bool)
                total := 0
                start := h.Buf.Start()
                for {
                    m, f, _ := h.Buf.FindNext(resp, start, h.Buf.End(), start, true, useRegex)
                    if !f {
                        break
                    }
                    total++
                    start = m[1]
                }
                InfoBar.Message(fmt.Sprintf("Found %d matches", total))
            } else {
                h.Cursor.ResetSelection()
                InfoBar.Message("No matches found")
            }
        } else {
            h.Cursor.ResetSelection()
        }
    }
    pattern := string(h.Cursor.GetSelection())
    if useRegex && pattern != "" {
        pattern = regexp.QuoteMeta(pattern)
    }
    if eventCallback != nil && pattern != "" {
        eventCallback(pattern)
    }
    InfoBar.Prompt(prompt, pattern, "Find", eventCallback, findCallback)
    if pattern != "" {
        InfoBar.SelectAll()
    }
    return true
}
func (h *BufPane) ToggleHighlightSearch() bool {
	h.Buf.HighlightSearch = !h.Buf.HighlightSearch
	return true
}
func (h *BufPane) UnhighlightSearch() bool {
	h.Buf.HighlightSearch = false
	return true
}
func (h *BufPane) FindNext() bool {
	searchLoc := h.Cursor.Loc
	if h.Cursor.HasSelection() {
		searchLoc = h.Cursor.CurSelection[1]
	}
	match, found, err := h.Buf.FindNext(h.Buf.LastSearch, h.Buf.Start(), h.Buf.End(), searchLoc, true, h.Buf.LastSearchRegex)
	if err != nil {
		InfoBar.Error(err)
	}
	if found {
		h.Cursor.SetSelectionStart(match[0])
		h.Cursor.SetSelectionEnd(match[1])
		h.Cursor.OrigSelection[0] = h.Cursor.CurSelection[0]
		h.Cursor.OrigSelection[1] = h.Cursor.CurSelection[1]
		h.GotoLoc(h.Cursor.CurSelection[1])
	} else {
		h.Cursor.ResetSelection()
	}
	return true
}
func (h *BufPane) FindPrevious() bool {
	searchLoc := h.Cursor.Loc
	if h.Cursor.HasSelection() {
		searchLoc = h.Cursor.CurSelection[0]
	}
	match, found, err := h.Buf.FindNext(h.Buf.LastSearch, h.Buf.Start(), h.Buf.End(), searchLoc, false, h.Buf.LastSearchRegex)
	if err != nil {
		InfoBar.Error(err)
	}
	if found {
		h.Cursor.SetSelectionStart(match[0])
		h.Cursor.SetSelectionEnd(match[1])
		h.Cursor.OrigSelection[0] = h.Cursor.CurSelection[0]
		h.Cursor.OrigSelection[1] = h.Cursor.CurSelection[1]
		h.GotoLoc(h.Cursor.CurSelection[1])
	} else {
		h.Cursor.ResetSelection()
	}
	return true
}
func (h *BufPane) DiffNext() bool {
	cur := h.Cursor.Loc.Y
	dl, err := h.Buf.FindNextDiffLine(cur, true)
	if err != nil {
		return false
	}
	h.GotoLoc(buffer.Loc{0, dl})
	return true
}
func (h *BufPane) DiffPrevious() bool {
	cur := h.Cursor.Loc.Y
	dl, err := h.Buf.FindNextDiffLine(cur, false)
	if err != nil {
		return false
	}
	h.GotoLoc(buffer.Loc{0, dl})
	return true
}
func (h *BufPane) Undo() bool {
	h.Buf.Undo()
	InfoBar.Message("Undid action")
	h.Relocate()
	return true
}
func (h *BufPane) Redo() bool {
	h.Buf.Redo()
	InfoBar.Message("Redid action")
	h.Relocate()
	return true
}
func (h *BufPane) Copy() bool {
	if h.Cursor.HasSelection() {
		h.Cursor.CopySelection(clipboard.ClipboardReg)
		h.freshClip = true
		InfoBar.Message("Copied selection")
	}
	h.Relocate()
	return true
}
func (h *BufPane) CopyLine() bool {
	if h.Cursor.HasSelection() {
		return false
	}
	origLoc := h.Cursor.Loc
	h.Cursor.SelectLine()
	h.Cursor.CopySelection(clipboard.ClipboardReg)
	h.freshClip = true
	InfoBar.Message("Copied line")
	h.Cursor.Deselect(true)
	h.Cursor.Loc = origLoc
	h.Relocate()
	return true
}
func (h *BufPane) CutLine() bool {
	h.Cursor.SelectLine()
	if !h.Cursor.HasSelection() {
		return false
	}
	if h.freshClip {
		if h.Cursor.HasSelection() {
			if clip, err := clipboard.Read(clipboard.ClipboardReg); err != nil {
				InfoBar.Error(err)
			} else {
				clipboard.WriteMulti(clip+string(h.Cursor.GetSelection()), clipboard.ClipboardReg, h.Cursor.Num, h.Buf.NumCursors())
			}
		}
	} else if time.Since(h.lastCutTime)/time.Second > 10*time.Second || !h.freshClip {
		h.Copy()
	}
	h.freshClip = true
	h.lastCutTime = time.Now()
	h.Cursor.DeleteSelection()
	h.Cursor.ResetSelection()
	InfoBar.Message("Cut line")
	h.Relocate()
	return true
}
func (h *BufPane) Cut() bool {
	if h.Cursor.HasSelection() {
		h.Cursor.CopySelection(clipboard.ClipboardReg)
		h.Cursor.DeleteSelection()
		h.Cursor.ResetSelection()
		h.freshClip = true
		InfoBar.Message("Cut selection")
		h.Relocate()
		return true
	}
	return h.CutLine()
}
func (h *BufPane) DuplicateLine() bool {
	var infoMessage = "Duplicated line"
	if h.Cursor.HasSelection() {
		infoMessage = "Duplicated selection"
		h.Buf.Insert(h.Cursor.CurSelection[1], string(h.Cursor.GetSelection()))
	} else {
		h.Cursor.End()
		h.Buf.Insert(h.Cursor.Loc, "\n"+string(h.Buf.LineBytes(h.Cursor.Y)))
	}
	InfoBar.Message(infoMessage)
	h.Relocate()
	return true
}
func (h *BufPane) DeleteLine() bool {
	h.Cursor.SelectLine()
	if !h.Cursor.HasSelection() {
		return false
	}
	h.Cursor.DeleteSelection()
	h.Cursor.ResetSelection()
	InfoBar.Message("Deleted line")
	h.Relocate()
	return true
}
func (h *BufPane) MoveLinesUp() bool {
	if h.Cursor.HasSelection() {
		if h.Cursor.CurSelection[0].Y == 0 {
			InfoBar.Message("Cannot move further up")
			return false
		}
		start := h.Cursor.CurSelection[0].Y
		end := h.Cursor.CurSelection[1].Y
		sel := 1
		if start > end {
			end, start = start, end
			sel = 0
		}
		compensate := false
		if h.Cursor.CurSelection[sel].X != 0 {
			end++
		} else {
			compensate = true
		}
		h.Buf.MoveLinesUp(
			start,
			end,
		)
		if compensate {
			h.Cursor.CurSelection[sel].Y -= 1
		}
	} else {
		if h.Cursor.Loc.Y == 0 {
			InfoBar.Message("Cannot move further up")
			return false
		}
		h.Buf.MoveLinesUp(
			h.Cursor.Loc.Y,
			h.Cursor.Loc.Y+1,
		)
	}
	h.Relocate()
	return true
}
func (h *BufPane) MoveLinesDown() bool {
	if h.Cursor.HasSelection() {
		if h.Cursor.CurSelection[1].Y >= h.Buf.LinesNum() {
			InfoBar.Message("Cannot move further down")
			return false
		}
		start := h.Cursor.CurSelection[0].Y
		end := h.Cursor.CurSelection[1].Y
		sel := 1
		if start > end {
			end, start = start, end
			sel = 0
		}
		if h.Cursor.CurSelection[sel].X != 0 {
			end++
		}
		h.Buf.MoveLinesDown(
			start,
			end,
		)
	} else {
		if h.Cursor.Loc.Y >= h.Buf.LinesNum()-1 {
			InfoBar.Message("Cannot move further down")
			return false
		}
		h.Buf.MoveLinesDown(
			h.Cursor.Loc.Y,
			h.Cursor.Loc.Y+1,
		)
	}
	h.Relocate()
	return true
}
func (h *BufPane) Paste() bool {
	clip, err := clipboard.ReadMulti(clipboard.ClipboardReg, h.Cursor.Num, h.Buf.NumCursors())
	if err != nil {
		InfoBar.Error(err)
	} else {
		h.paste(clip)
	}
	h.Relocate()
	return true
}
func (h *BufPane) PastePrimary() bool {
	clip, err := clipboard.ReadMulti(clipboard.PrimaryReg, h.Cursor.Num, h.Buf.NumCursors())
	if err != nil {
		InfoBar.Error(err)
	} else {
		h.paste(clip)
	}
	h.Relocate()
	return true
}
func (h *BufPane) paste(clip string) {
	if h.Buf.Settings["smartpaste"].(bool) {
		if h.Cursor.X > 0 {
			leadingPasteWS := string(util.GetLeadingWhitespace([]byte(clip)))
			if leadingPasteWS != " " && strings.Contains(clip, "\n"+leadingPasteWS) {
				leadingWS := string(util.GetLeadingWhitespace(h.Buf.LineBytes(h.Cursor.Y)))
				clip = strings.TrimPrefix(clip, leadingPasteWS)
				clip = strings.ReplaceAll(clip, "\n"+leadingPasteWS, "\n"+leadingWS)
			}
		}
	}
	if h.Cursor.HasSelection() {
		h.Cursor.DeleteSelection()
		h.Cursor.ResetSelection()
	}
	h.Buf.Insert(h.Cursor.Loc, clip)
	h.freshClip = false
	InfoBar.Message("Pasted clipboard")
}
func (h *BufPane) JumpToMatchingBrace() bool {
	for _, bp := range buffer.BracePairs {
		r := h.Cursor.RuneUnder(h.Cursor.X)
		rl := h.Cursor.RuneUnder(h.Cursor.X - 1)
		if r == bp[0] || r == bp[1] || rl == bp[0] || rl == bp[1] {
			matchingBrace, left, found := h.Buf.FindMatchingBrace(bp, h.Cursor.Loc)
			if found {
				if left {
					h.Cursor.GotoLoc(matchingBrace)
				} else {
					h.Cursor.GotoLoc(matchingBrace.Move(1, h.Buf))
				}
				h.Relocate()
				return true
			}
		}
	}
	return false
}
func (h *BufPane) SelectAll() bool {
	h.Cursor.SetSelectionStart(h.Buf.Start())
	h.Cursor.SetSelectionEnd(h.Buf.End())
	h.Cursor.X = 0
	h.Cursor.Y = 0
	h.Relocate()
	return true
}
func (h *BufPane) OpenFile() bool {
	InfoBar.Prompt("> ", "open ", "Open", nil, func(resp string, canceled bool) {
		if !canceled {
			h.HandleCommand(resp)
		}
	})
	return true
}
func (h *BufPane) JumpLine() bool {
	InfoBar.Prompt("> ", "goto ", "Command", nil, func(resp string, canceled bool) {
		if !canceled {
			h.HandleCommand(resp)
		}
	})
	return true
}
func (h *BufPane) Start() bool {
	v := h.GetView()
	v.StartLine = display.SLoc{0, 0}
	h.SetView(v)
	return true
}
func (h *BufPane) End() bool {
	v := h.GetView()
	v.StartLine = h.Scroll(h.SLocFromLoc(h.Buf.End()), -h.BufView().Height+1)
	h.SetView(v)
	return true
}
func (h *BufPane) PageUp() bool {
	h.ScrollUp(h.BufView().Height)
	return true
}
func (h *BufPane) PageDown() bool {
	h.ScrollDown(h.BufView().Height)
	h.ScrollAdjust()
	return true
}
func (h *BufPane) SelectPageUp() bool {
	if !h.Cursor.HasSelection() {
		h.Cursor.OrigSelection[0] = h.Cursor.Loc
	}
	h.MoveCursorUp(h.BufView().Height)
	h.Cursor.SelectTo(h.Cursor.Loc)
	h.Relocate()
	return true
}
func (h *BufPane) SelectPageDown() bool {
	if !h.Cursor.HasSelection() {
		h.Cursor.OrigSelection[0] = h.Cursor.Loc
	}
	h.MoveCursorDown(h.BufView().Height)
	h.Cursor.SelectTo(h.Cursor.Loc)
	h.Relocate()
	return true
}
func (h *BufPane) CursorPageUp() bool {
	h.Cursor.Deselect(true)
	if h.Cursor.HasSelection() {
		h.Cursor.Loc = h.Cursor.CurSelection[0]
		h.Cursor.ResetSelection()
		h.Cursor.StoreVisualX()
	}
	h.MoveCursorUp(h.BufView().Height)
	h.Relocate()
	return true
}
func (h *BufPane) CursorPageDown() bool {
	h.Cursor.Deselect(false)
	if h.Cursor.HasSelection() {
		h.Cursor.Loc = h.Cursor.CurSelection[1]
		h.Cursor.ResetSelection()
		h.Cursor.StoreVisualX()
	}
	h.MoveCursorDown(h.BufView().Height)
	h.Relocate()
	return true
}
func (h *BufPane) HalfPageUp() bool {
	h.ScrollUp(h.BufView().Height / 2)
	return true
}
func (h *BufPane) HalfPageDown() bool {
	h.ScrollDown(h.BufView().Height / 2)
	h.ScrollAdjust()
	return true
}
func (h *BufPane) ToggleDiffGutter() bool {
	if !h.Buf.Settings["diffgutter"].(bool) {
		h.Buf.Settings["diffgutter"] = true
		h.Buf.UpdateDiff(func(synchronous bool) {
			screen.Redraw()
		})
		InfoBar.Message("Enabled diff gutter")
	} else {
		h.Buf.Settings["diffgutter"] = false
		InfoBar.Message("Disabled diff gutter")
	}
	return true
}
func (h *BufPane) ToggleRuler() bool {
	if !h.Buf.Settings["ruler"].(bool) {
		h.Buf.Settings["ruler"] = true
		InfoBar.Message("Enabled ruler")
	} else {
		h.Buf.Settings["ruler"] = false
		InfoBar.Message("Disabled ruler")
	}
	return true
}
func (h *BufPane) ClearStatus() bool {
	InfoBar.Message("")
	return true
}
func (h *BufPane) ToggleHelp() bool {
	if h.Buf.Type == buffer.BTHelp {
		h.Quit()
	} else {
		h.openHelp("help")
	}
	return true
}
func (h *BufPane) ToggleKeyMenu() bool {
	config.GlobalSettings["keymenu"] = !config.GetGlobalOption("keymenu").(bool)
	Tabs.Resize()
	return true
}
func (h *BufPane) ShellMode() bool {
	InfoBar.Prompt("$ ", "", "Shell", nil, func(resp string, canceled bool) {
		if !canceled {
			shell.RunInteractiveShell(resp, true, false)
		}
	})
	return true
}
func (h *BufPane) CommandMode() bool {
	InfoBar.Prompt("> ", "", "Command", nil, func(resp string, canceled bool) {
		if !canceled {
			h.HandleCommand(resp)
		}
	})
	return true
}
func (h *BufPane) ToggleOverwriteMode() bool {
	h.isOverwriteMode = !h.isOverwriteMode
	return true
}
func (h *BufPane) Escape() bool {
	return true
}
func (h *BufPane) Deselect() bool {
	h.Cursor.Deselect(true)
	return true
}
func (h *BufPane) ClearInfo() bool {
	InfoBar.Message("")
	return true
}
func (h *BufPane) ForceQuit() bool {
	h.Buf.Close()
	if len(MainTab().Panes) > 1 {
		h.Unsplit()
	} else if len(Tabs.List) > 1 {
		Tabs.RemoveTab(h.splitID)
	} else {
		screen.Screen.Fini()
		InfoBar.Close()
		runtime.Goexit()
	}
	return true
}
func (h *BufPane) Quit() bool {
	if h.Buf.Modified() {
		if config.GlobalSettings["autosave"].(float64) > 0 {
			h.SaveCB("Quit", func() {
				h.ForceQuit()
			})
		} else {
			InfoBar.YNPrompt("Save changes to "+h.Buf.GetName()+" before closing? (y,n,esc)", func(yes, canceled bool) {
				if !canceled && !yes {
					h.ForceQuit()
				} else if !canceled && yes {
					h.SaveCB("Quit", func() {
						h.ForceQuit()
					})
				}
			})
		}
	} else {
		h.ForceQuit()
	}
	return true
}
func (h *BufPane) QuitAll() bool {
	anyModified := false
	for _, b := range buffer.OpenBuffers {
		if b.Modified() {
			anyModified = true
			break
		}
	}
	quit := func() {
		buffer.CloseOpenBuffers()
		screen.Screen.Fini()
		InfoBar.Close()
		runtime.Goexit()
	}
	if anyModified {
		InfoBar.YNPrompt("Quit mecro? (all open buffers will be closed without saving)", func(yes, canceled bool) {
			if !canceled && yes {
				quit()
			}
		})
	} else {
		quit()
	}
	return true
}
func (h *BufPane) AddTab() bool {
	width, height := screen.Screen.Size()
	iOffset := config.GetInfoBarOffset()
	b := buffer.NewBufferFromString("", "", buffer.BTDefault)
	tp := NewTabFromBuffer(0, 0, width, height-iOffset, b)
	Tabs.AddTab(tp)
	Tabs.SetActive(len(Tabs.List) - 1)
	return true
}
func (h *BufPane) PreviousTab() bool {
	tabsLen := len(Tabs.List)
	a := Tabs.Active() + tabsLen
	Tabs.SetActive((a - 1) % tabsLen)
	return true
}
func (h *BufPane) NextTab() bool {
	a := Tabs.Active()
	Tabs.SetActive((a + 1) % len(Tabs.List))
	return true
}
func (h *BufPane) VSplitAction() bool {
	h.VSplitBuf(buffer.NewBufferFromString("", "", buffer.BTDefault))
	return true
}
func (h *BufPane) HSplitAction() bool {
	h.HSplitBuf(buffer.NewBufferFromString("", "", buffer.BTDefault))
	return true
}
func (h *BufPane) Unsplit() bool {
	tab := h.tab
	n := tab.GetNode(h.splitID)
	ok := n.Unsplit()
	if ok {
		tab.RemovePane(tab.GetPane(h.splitID))
		tab.Resize()
		tab.SetActive(len(tab.Panes) - 1)
		return true
	}
	return false
}
func (h *BufPane) NextSplit() bool {
	a := h.tab.active
	if a < len(h.tab.Panes)-1 {
		a++
	} else {
		a = 0
	}
	h.tab.SetActive(a)
	return true
}
func (h *BufPane) PreviousSplit() bool {
	a := h.tab.active
	if a > 0 {
		a--
	} else {
		a = len(h.tab.Panes) - 1
	}
	h.tab.SetActive(a)
	return true
}
var curmacro []interface{}
var recordingMacro bool
func (h *BufPane) ToggleMacro() bool {
	recordingMacro = !recordingMacro
	if recordingMacro {
		curmacro = []interface{}{}
		InfoBar.Message("Recording")
	} else {
		InfoBar.Message("Stopped recording")
	}
	h.Relocate()
	return true
}
func (h *BufPane) PlayMacro() bool {
	if recordingMacro {
		return false
	}
	for _, action := range curmacro {
		switch t := action.(type) {
		case rune:
			h.DoRuneInsert(t)
		case BufKeyAction:
			t(h)
		}
	}
	h.Relocate()
	return true
}
func (h *BufPane) SpawnMultiCursor() bool {
	spawner := h.Buf.GetCursor(h.Buf.NumCursors() - 1)
	if !spawner.HasSelection() {
		spawner.SelectWord()
		h.multiWord = true
		h.Relocate()
		return true
	}
	sel := spawner.GetSelection()
	searchStart := spawner.CurSelection[1]
	search := string(sel)
	search = regexp.QuoteMeta(search)
	if h.multiWord {
		search = "\\b" + search + "\\b"
	}
	match, found, err := h.Buf.FindNext(search, h.Buf.Start(), h.Buf.End(), searchStart, true, true)
	if err != nil {
		InfoBar.Error(err)
	}
	if found {
		c := buffer.NewCursor(h.Buf, buffer.Loc{})
		c.SetSelectionStart(match[0])
		c.SetSelectionEnd(match[1])
		c.OrigSelection[0] = c.CurSelection[0]
		c.OrigSelection[1] = c.CurSelection[1]
		c.Loc = c.CurSelection[1]
		h.Buf.AddCursor(c)
		h.Buf.SetCurCursor(h.Buf.NumCursors() - 1)
		h.Buf.MergeCursors()
	} else {
		InfoBar.Message("No matches found")
	}
	h.Relocate()
	return true
}
func (h *BufPane) SpawnMultiCursorUpN(n int) bool {
	lastC := h.Buf.GetCursor(h.Buf.NumCursors() - 1)
	var c *buffer.Cursor
	if !h.Buf.Settings["softwrap"].(bool) {
		if n > 0 && lastC.Y == 0 {
			return false
		}
		if n < 0 && lastC.Y+1 == h.Buf.LinesNum() {
			return false
		}
		h.Buf.DeselectCursors()
		c = buffer.NewCursor(h.Buf, buffer.Loc{lastC.X, lastC.Y - n})
		c.LastVisualX = lastC.LastVisualX
		c.X = c.GetCharPosInLine(h.Buf.LineBytes(c.Y), c.LastVisualX)
		c.Relocate()
	} else {
		vloc := h.VLocFromLoc(lastC.Loc)
		sloc := h.Scroll(vloc.SLoc, -n)
		if sloc == vloc.SLoc {
			return false
		}
		h.Buf.DeselectCursors()
		vloc.SLoc = sloc
		vloc.VisualX = lastC.LastVisualX
		c = buffer.NewCursor(h.Buf, h.LocFromVLoc(vloc))
		c.LastVisualX = lastC.LastVisualX
	}
	h.Buf.AddCursor(c)
	h.Buf.SetCurCursor(h.Buf.NumCursors() - 1)
	h.Buf.MergeCursors()
	h.Relocate()
	return true
}
func (h *BufPane) SpawnMultiCursorUp() bool {
	return h.SpawnMultiCursorUpN(1)
}
func (h *BufPane) SpawnMultiCursorDown() bool {
	return h.SpawnMultiCursorUpN(-1)
}
func (h *BufPane) SpawnMultiCursorSelect() bool {
	if h.Buf.NumCursors() > 1 {
		return false
	}
	var startLine int
	var endLine int
	a, b := h.Cursor.CurSelection[0].Y, h.Cursor.CurSelection[1].Y
	if a > b {
		startLine, endLine = b, a
	} else {
		startLine, endLine = a, b
	}
	if h.Cursor.HasSelection() {
		h.Cursor.ResetSelection()
		h.Cursor.GotoLoc(buffer.Loc{0, startLine})
		for i := startLine; i <= endLine; i++ {
			c := buffer.NewCursor(h.Buf, buffer.Loc{0, i})
			c.StoreVisualX()
			h.Buf.AddCursor(c)
		}
		h.Buf.MergeCursors()
	} else {
		return false
	}
	InfoBar.Message("Added cursors from selection")
	return true
}
func (h *BufPane) MouseMultiCursor(e *tcell.EventMouse) bool {
	b := h.Buf
	mx, my := e.Position()
	if my >= h.BufView().Y+h.BufView().Height {
		return false
	}
	mouseLoc := h.LocFromVisual(buffer.Loc{X: mx, Y: my})
	if h.Buf.NumCursors() > 1 {
		cursors := h.Buf.GetCursors()
		for _, c := range cursors {
			if c.Loc == mouseLoc {
				h.Buf.RemoveCursor(c.Num)
				return true
			}
		}
	}
	c := buffer.NewCursor(b, mouseLoc)
	b.AddCursor(c)
	b.MergeCursors()
	return true
}
func (h *BufPane) SkipMultiCursor() bool {
	lastC := h.Buf.GetCursor(h.Buf.NumCursors() - 1)
	sel := lastC.GetSelection()
	searchStart := lastC.CurSelection[1]
	search := string(sel)
	search = regexp.QuoteMeta(search)
	if h.multiWord {
		search = "\\b" + search + "\\b"
	}
	match, found, err := h.Buf.FindNext(search, h.Buf.Start(), h.Buf.End(), searchStart, true, true)
	if err != nil {
		InfoBar.Error(err)
	}
	if found {
		lastC.SetSelectionStart(match[0])
		lastC.SetSelectionEnd(match[1])
		lastC.OrigSelection[0] = lastC.CurSelection[0]
		lastC.OrigSelection[1] = lastC.CurSelection[1]
		lastC.Loc = lastC.CurSelection[1]
		h.Buf.MergeCursors()
		h.Buf.SetCurCursor(h.Buf.NumCursors() - 1)
	} else {
		InfoBar.Message("No matches found")
	}
	h.Relocate()
	return true
}
func (h *BufPane) RemoveMultiCursor() bool {
	if h.Buf.NumCursors() > 1 {
		h.Buf.RemoveCursor(h.Buf.NumCursors() - 1)
		h.Buf.SetCurCursor(h.Buf.NumCursors() - 1)
		h.Buf.UpdateCursors()
	} else {
		h.multiWord = false
	}
	h.Relocate()
	return true
}
func (h *BufPane) RemoveAllMultiCursors() bool {
	h.Buf.ClearCursors()
	h.multiWord = false
	h.Relocate()
	return true
}
func (h *BufPane) None() bool {
	return true
}
