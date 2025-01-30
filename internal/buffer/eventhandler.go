package buffer
import (
	"bytes"
	"time"
	dmp "github.com/sergi/go-diff/diffmatchpatch"
	"github.com/zyedidia/micro/v2/internal/config"
	ulua "github.com/zyedidia/micro/v2/internal/lua"
	"github.com/zyedidia/micro/v2/internal/screen"
	"github.com/zyedidia/micro/v2/internal/util"
	luar "layeh.com/gopher-luar"
)
const (
	TextEventInsert = 1
	TextEventRemove = -1
	TextEventReplace = 0
	undoThreshold = 1000
)
type TextEvent struct {
	C Cursor
	EventType int
	Deltas    []Delta
	Time      time.Time
}
type Delta struct {
	Text  []byte
	Start Loc
	End   Loc
}
func (eh *EventHandler) DoTextEvent(t *TextEvent, useUndo bool) {
	oldl := eh.buf.LinesNum()
	if useUndo {
		eh.Execute(t)
	} else {
		ExecuteTextEvent(t, eh.buf)
	}
	if len(t.Deltas) != 1 {
		return
	}
	text := t.Deltas[0].Text
	start := t.Deltas[0].Start
	lastnl := -1
	var endX int
	var textX int
	if t.EventType == TextEventInsert {
		linecount := eh.buf.LinesNum() - oldl
		textcount := util.CharacterCount(text)
		lastnl = bytes.LastIndex(text, []byte{'\n'})
		if lastnl >= 0 {
			endX = util.CharacterCount(text[lastnl+1:])
			textX = endX
		} else {
			endX = start.X + textcount
			textX = textcount
		}
		t.Deltas[0].End = clamp(Loc{endX, start.Y + linecount}, eh.buf.LineArray)
	}
	end := t.Deltas[0].End
	for _, c := range eh.cursors {
		move := func(loc Loc) Loc {
			if t.EventType == TextEventInsert {
				if start.Y != loc.Y && loc.GreaterThan(start) {
					loc.Y += end.Y - start.Y
				} else if loc.Y == start.Y && loc.GreaterEqual(start) {
					loc.Y += end.Y - start.Y
					if lastnl >= 0 {
						loc.X += textX - start.X
					} else {
						loc.X += textX
					}
				}
				return loc
			} else {
				if loc.Y != end.Y && loc.GreaterThan(end) {
					loc.Y -= end.Y - start.Y
				} else if loc.Y == end.Y && loc.GreaterEqual(end) {
					loc = loc.MoveLA(-DiffLA(start, end, eh.buf.LineArray), eh.buf.LineArray)
				}
				return loc
			}
		}
		c.Loc = move(c.Loc)
		c.CurSelection[0] = move(c.CurSelection[0])
		c.CurSelection[1] = move(c.CurSelection[1])
		c.OrigSelection[0] = move(c.OrigSelection[0])
		c.OrigSelection[1] = move(c.OrigSelection[1])
		c.Relocate()
		c.LastVisualX = c.GetVisualX()
	}
	if useUndo {
		eh.updateTrailingWs(t)
	}
}
func ExecuteTextEvent(t *TextEvent, buf *SharedBuffer) {
	if t.EventType == TextEventInsert {
		for _, d := range t.Deltas {
			buf.insert(d.Start, d.Text)
		}
	} else if t.EventType == TextEventRemove {
		for i, d := range t.Deltas {
			t.Deltas[i].Text = buf.remove(d.Start, d.End)
		}
	} else if t.EventType == TextEventReplace {
		for i, d := range t.Deltas {
			t.Deltas[i].Text = buf.remove(d.Start, d.End)
			buf.insert(d.Start, d.Text)
			t.Deltas[i].Start = d.Start
			t.Deltas[i].End = Loc{d.Start.X + util.CharacterCount(d.Text), d.Start.Y}
		}
		for i, j := 0, len(t.Deltas)-1; i < j; i, j = i+1, j-1 {
			t.Deltas[i], t.Deltas[j] = t.Deltas[j], t.Deltas[i]
		}
	}
}
func (eh *EventHandler) UndoTextEvent(t *TextEvent) {
	t.EventType = -t.EventType
	eh.DoTextEvent(t, false)
}
type EventHandler struct {
	buf       *SharedBuffer
	cursors   []*Cursor
	active    int
	UndoStack *TEStack
	RedoStack *TEStack
}
func NewEventHandler(buf *SharedBuffer, cursors []*Cursor) *EventHandler {
	eh := new(EventHandler)
	eh.UndoStack = new(TEStack)
	eh.RedoStack = new(TEStack)
	eh.buf = buf
	eh.cursors = cursors
	return eh
}
func (eh *EventHandler) ApplyDiff(new string) {
	differ := dmp.New()
	diff := differ.DiffMain(string(eh.buf.Bytes()), new, false)
	loc := eh.buf.Start()
	for _, d := range diff {
		if d.Type == dmp.DiffDelete {
			eh.Remove(loc, loc.MoveLA(util.CharacterCountInString(d.Text), eh.buf.LineArray))
		} else {
			if d.Type == dmp.DiffInsert {
				eh.Insert(loc, d.Text)
			}
			loc = loc.MoveLA(util.CharacterCountInString(d.Text), eh.buf.LineArray)
		}
	}
}
func (eh *EventHandler) Insert(start Loc, textStr string) {
	text := []byte(textStr)
	eh.InsertBytes(start, text)
}
func (eh *EventHandler) InsertBytes(start Loc, text []byte) {
	if len(text) == 0 {
		return
	}
	start = clamp(start, eh.buf.LineArray)
	e := &TextEvent{
		C:         *eh.cursors[eh.active],
		EventType: TextEventInsert,
		Deltas:    []Delta{{text, start, Loc{0, 0}}},
		Time:      time.Now(),
	}
	eh.DoTextEvent(e, true)
}
func (eh *EventHandler) Remove(start, end Loc) {
	if start == end {
		return
	}
	start = clamp(start, eh.buf.LineArray)
	end = clamp(end, eh.buf.LineArray)
	e := &TextEvent{
		C:         *eh.cursors[eh.active],
		EventType: TextEventRemove,
		Deltas:    []Delta{{[]byte{}, start, end}},
		Time:      time.Now(),
	}
	eh.DoTextEvent(e, true)
}
func (eh *EventHandler) MultipleReplace(deltas []Delta) {
	e := &TextEvent{
		C:         *eh.cursors[eh.active],
		EventType: TextEventReplace,
		Deltas:    deltas,
		Time:      time.Now(),
	}
	eh.Execute(e)
}
func (eh *EventHandler) Replace(start, end Loc, replace string) {
	eh.Remove(start, end)
	eh.Insert(start, replace)
}
func (eh *EventHandler) Execute(t *TextEvent) {
	if eh.RedoStack.Len() > 0 {
		eh.RedoStack = new(TEStack)
	}
	eh.UndoStack.Push(t)
	b, err := config.RunPluginFnBool(nil, "onBeforeTextEvent", luar.New(ulua.L, eh.buf), luar.New(ulua.L, t))
	if err != nil {
		screen.TermMessage(err)
	}
	if !b {
		return
	}
	ExecuteTextEvent(t, eh.buf)
}
func (eh *EventHandler) Undo() {
	t := eh.UndoStack.Peek()
	if t == nil {
		return
	}
	startTime := t.Time.UnixNano() / int64(time.Millisecond)
	endTime := startTime - (startTime % undoThreshold)
	for {
		t = eh.UndoStack.Peek()
		if t == nil {
			return
		}
		if t.Time.UnixNano()/int64(time.Millisecond) < endTime {
			return
		}
		eh.UndoOneEvent()
	}
}
func (eh *EventHandler) UndoOneEvent() {
	t := eh.UndoStack.Pop()
	if t == nil {
		return
	}
	eh.UndoTextEvent(t)
	teCursor := t.C
	if teCursor.Num >= 0 && teCursor.Num < len(eh.cursors) {
		t.C = *eh.cursors[teCursor.Num]
		eh.cursors[teCursor.Num].Goto(teCursor)
		eh.cursors[teCursor.Num].NewTrailingWsY = teCursor.NewTrailingWsY
	} else {
		teCursor.Num = -1
	}
	eh.RedoStack.Push(t)
}
func (eh *EventHandler) Redo() {
	t := eh.RedoStack.Peek()
	if t == nil {
		return
	}
	startTime := t.Time.UnixNano() / int64(time.Millisecond)
	endTime := startTime - (startTime % undoThreshold) + undoThreshold
	for {
		t = eh.RedoStack.Peek()
		if t == nil {
			return
		}
		if t.Time.UnixNano()/int64(time.Millisecond) > endTime {
			return
		}
		eh.RedoOneEvent()
	}
}
func (eh *EventHandler) RedoOneEvent() {
	t := eh.RedoStack.Pop()
	if t == nil {
		return
	}
	teCursor := t.C
	if teCursor.Num >= 0 && teCursor.Num < len(eh.cursors) {
		t.C = *eh.cursors[teCursor.Num]
		eh.cursors[teCursor.Num].Goto(teCursor)
		eh.cursors[teCursor.Num].NewTrailingWsY = teCursor.NewTrailingWsY
	} else {
		teCursor.Num = -1
	}
	eh.UndoTextEvent(t)
	eh.UndoStack.Push(t)
}
func (eh *EventHandler) updateTrailingWs(t *TextEvent) {
	if len(t.Deltas) != 1 {
		return
	}
	text := t.Deltas[0].Text
	start := t.Deltas[0].Start
	end := t.Deltas[0].End
	c := eh.cursors[eh.active]
	isEol := func(loc Loc) bool {
		return loc.X == util.CharacterCount(eh.buf.LineBytes(loc.Y))
	}
	if t.EventType == TextEventInsert && c.Loc == end && isEol(end) {
		var addedTrailingWs bool
		addedAfterWs := false
		addedWsOnly := false
		if start.Y == end.Y {
			addedTrailingWs = util.HasTrailingWhitespace(text)
			addedWsOnly = util.IsBytesWhitespace(text)
			addedAfterWs = start.X > 0 && util.IsWhitespace(c.buf.RuneAt(Loc{start.X - 1, start.Y}))
		} else {
			lastnl := bytes.LastIndex(text, []byte{'\n'})
			addedTrailingWs = util.HasTrailingWhitespace(text[lastnl+1:])
		}
		if addedTrailingWs && !(addedAfterWs && addedWsOnly) {
			c.NewTrailingWsY = c.Y
		} else if !addedTrailingWs {
			c.NewTrailingWsY = -1
		}
	} else if t.EventType == TextEventRemove && c.Loc == start && isEol(start) {
		removedAfterWs := util.HasTrailingWhitespace(eh.buf.LineBytes(start.Y))
		var removedWsOnly bool
		if start.Y == end.Y {
			removedWsOnly = util.IsBytesWhitespace(text)
		} else {
			firstnl := bytes.Index(text, []byte{'\n'})
			removedWsOnly = util.IsBytesWhitespace(text[:firstnl])
		}
		if removedAfterWs && !removedWsOnly {
			c.NewTrailingWsY = c.Y
		} else if !removedAfterWs {
			c.NewTrailingWsY = -1
		}
	} else if c.NewTrailingWsY != -1 && start.Y != end.Y && c.Loc.GreaterThan(start) &&
		((t.EventType == TextEventInsert && c.Y == c.NewTrailingWsY+(end.Y-start.Y)) ||
			(t.EventType == TextEventRemove && c.Y == c.NewTrailingWsY-(end.Y-start.Y))) {
		c.NewTrailingWsY = c.Y
	}
}