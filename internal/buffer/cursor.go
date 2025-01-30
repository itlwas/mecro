package buffer
import (
	"github.com/zyedidia/micro/v2/internal/clipboard"
	"github.com/zyedidia/micro/v2/internal/util"
)
func InBounds(pos Loc, buf *Buffer) bool {
	if pos.Y < 0 || pos.Y >= len(buf.lines) || pos.X < 0 || pos.X > util.CharacterCount(buf.LineBytes(pos.Y)) {
		return false
	}
	return true
}
type Cursor struct {
	buf *Buffer
	Loc
	LastVisualX int
	CurSelection [2]Loc
	OrigSelection [2]Loc
	NewTrailingWsY int
	Num int
}
func NewCursor(b *Buffer, l Loc) *Cursor {
	c := &Cursor{
		buf: b,
		Loc: l,
		NewTrailingWsY: -1,
	}
	c.StoreVisualX()
	return c
}
func (c *Cursor) SetBuf(b *Buffer) {
	c.buf = b
}
func (c *Cursor) Buf() *Buffer {
	return c.buf
}
func (c *Cursor) Goto(b Cursor) {
	c.X, c.Y, c.LastVisualX = b.X, b.Y, b.LastVisualX
	c.OrigSelection, c.CurSelection = b.OrigSelection, b.CurSelection
}
func (c *Cursor) GotoLoc(l Loc) {
	c.X, c.Y = l.X, l.Y
	c.StoreVisualX()
}
func (c *Cursor) GetVisualX() int {
	if c.buf.GetVisualX != nil {
		return c.buf.GetVisualX(c.Loc)
	}
	if c.X <= 0 {
		c.X = 0
		return 0
	}
	bytes := c.buf.LineBytes(c.Y)
	tabsize := int(c.buf.Settings["tabsize"].(float64))
	return util.StringWidth(bytes, c.X, tabsize)
}
func (c *Cursor) GetCharPosInLine(b []byte, visualPos int) int {
	tabsize := int(c.buf.Settings["tabsize"].(float64))
	return util.GetCharPosInLine(b, visualPos, tabsize)
}
func (c *Cursor) Start() {
	c.X = 0
	c.LastVisualX = c.GetVisualX()
}
func (c *Cursor) StartOfText() {
	c.Start()
	for util.IsWhitespace(c.RuneUnder(c.X)) {
		if c.X == util.CharacterCount(c.buf.LineBytes(c.Y)) {
			break
		}
		c.Right()
	}
}
func (c *Cursor) IsStartOfText() bool {
	x := 0
	for util.IsWhitespace(c.RuneUnder(x)) {
		if x == util.CharacterCount(c.buf.LineBytes(c.Y)) {
			break
		}
		x++
	}
	return c.X == x
}
func (c *Cursor) End() {
	c.X = util.CharacterCount(c.buf.LineBytes(c.Y))
	c.LastVisualX = c.GetVisualX()
}
func (c *Cursor) CopySelection(target clipboard.Register) {
	if c.HasSelection() {
		if target != clipboard.PrimaryReg || c.buf.Settings["useprimary"].(bool) {
			clipboard.WriteMulti(string(c.GetSelection()), target, c.Num, c.buf.NumCursors())
		}
	}
}
func (c *Cursor) ResetSelection() {
	c.CurSelection[0] = c.buf.Start()
	c.CurSelection[1] = c.buf.Start()
}
func (c *Cursor) SetSelectionStart(pos Loc) {
	c.CurSelection[0] = pos
}
func (c *Cursor) SetSelectionEnd(pos Loc) {
	c.CurSelection[1] = pos
}
func (c *Cursor) HasSelection() bool {
	return c.CurSelection[0] != c.CurSelection[1]
}
func (c *Cursor) DeleteSelection() {
	if c.CurSelection[0].GreaterThan(c.CurSelection[1]) {
		c.buf.Remove(c.CurSelection[1], c.CurSelection[0])
		c.Loc = c.CurSelection[1]
	} else if !c.HasSelection() {
		return
	} else {
		c.buf.Remove(c.CurSelection[0], c.CurSelection[1])
		c.Loc = c.CurSelection[0]
	}
}
func (c *Cursor) Deselect(start bool) {
	if c.HasSelection() {
		if start {
			c.Loc = c.CurSelection[0]
		} else {
			c.Loc = c.CurSelection[1].Move(-1, c.buf)
		}
		c.ResetSelection()
		c.StoreVisualX()
	}
}
func (c *Cursor) GetSelection() []byte {
	if InBounds(c.CurSelection[0], c.buf) && InBounds(c.CurSelection[1], c.buf) {
		if c.CurSelection[0].GreaterThan(c.CurSelection[1]) {
			return c.buf.Substr(c.CurSelection[1], c.CurSelection[0])
		}
		return c.buf.Substr(c.CurSelection[0], c.CurSelection[1])
	}
	return []byte{}
}
func (c *Cursor) SelectLine() {
	c.Start()
	c.SetSelectionStart(c.Loc)
	c.End()
	if len(c.buf.lines)-1 > c.Y {
		c.SetSelectionEnd(c.Loc.Move(1, c.buf))
	} else {
		c.SetSelectionEnd(c.Loc)
	}
	c.OrigSelection = c.CurSelection
}
func (c *Cursor) AddLineToSelection() {
	if c.Loc.LessThan(c.OrigSelection[0]) {
		c.Start()
		c.SetSelectionStart(c.Loc)
		c.SetSelectionEnd(c.OrigSelection[1])
	}
	if c.Loc.GreaterThan(c.OrigSelection[1]) {
		c.End()
		c.SetSelectionEnd(c.Loc.Move(1, c.buf))
		c.SetSelectionStart(c.OrigSelection[0])
	}
	if c.Loc.LessThan(c.OrigSelection[1]) && c.Loc.GreaterThan(c.OrigSelection[0]) {
		c.CurSelection = c.OrigSelection
	}
}
func (c *Cursor) UpN(amount int) {
	proposedY := c.Y - amount
	if proposedY < 0 {
		proposedY = 0
	} else if proposedY >= len(c.buf.lines) {
		proposedY = len(c.buf.lines) - 1
	}
	bytes := c.buf.LineBytes(proposedY)
	c.X = c.GetCharPosInLine(bytes, c.LastVisualX)
	if c.X > util.CharacterCount(bytes) || (amount < 0 && proposedY == c.Y) {
		c.X = util.CharacterCount(bytes)
		c.StoreVisualX()
	}
	if c.X < 0 || (amount > 0 && proposedY == c.Y) {
		c.X = 0
		c.StoreVisualX()
	}
	c.Y = proposedY
}
func (c *Cursor) DownN(amount int) {
	c.UpN(-amount)
}
func (c *Cursor) Up() {
	c.UpN(1)
}
func (c *Cursor) Down() {
	c.DownN(1)
}
func (c *Cursor) Left() {
	if c.Loc == c.buf.Start() {
		return
	}
	if c.X > 0 {
		c.X--
	} else {
		c.Up()
		c.End()
	}
	c.StoreVisualX()
}
func (c *Cursor) Right() {
	if c.Loc == c.buf.End() {
		return
	}
	if c.X < util.CharacterCount(c.buf.LineBytes(c.Y)) {
		c.X++
	} else {
		c.Down()
		c.Start()
	}
	c.StoreVisualX()
}
func (c *Cursor) Relocate() {
	if c.Y < 0 {
		c.Y = 0
	} else if c.Y >= len(c.buf.lines) {
		c.Y = len(c.buf.lines) - 1
	}
	if c.X < 0 {
		c.X = 0
	} else if c.X > util.CharacterCount(c.buf.LineBytes(c.Y)) {
		c.X = util.CharacterCount(c.buf.LineBytes(c.Y))
	}
}
func (c *Cursor) SelectWord() {
	if len(c.buf.LineBytes(c.Y)) == 0 {
		return
	}
	if !util.IsWordChar(c.RuneUnder(c.X)) {
		c.SetSelectionStart(c.Loc)
		c.SetSelectionEnd(c.Loc.Move(1, c.buf))
		c.OrigSelection = c.CurSelection
		return
	}
	forward, backward := c.X, c.X
	for backward > 0 && util.IsWordChar(c.RuneUnder(backward-1)) {
		backward--
	}
	c.SetSelectionStart(Loc{backward, c.Y})
	c.OrigSelection[0] = c.CurSelection[0]
	lineLen := util.CharacterCount(c.buf.LineBytes(c.Y)) - 1
	for forward < lineLen && util.IsWordChar(c.RuneUnder(forward+1)) {
		forward++
	}
	c.SetSelectionEnd(Loc{forward, c.Y}.Move(1, c.buf))
	c.OrigSelection[1] = c.CurSelection[1]
	c.Loc = c.CurSelection[1]
}
func (c *Cursor) AddWordToSelection() {
	if c.Loc.GreaterThan(c.OrigSelection[0]) && c.Loc.LessThan(c.OrigSelection[1]) {
		c.CurSelection = c.OrigSelection
		return
	}
	if c.Loc.LessThan(c.OrigSelection[0]) {
		backward := c.X
		for backward > 0 && util.IsWordChar(c.RuneUnder(backward-1)) {
			backward--
		}
		c.SetSelectionStart(Loc{backward, c.Y})
		c.SetSelectionEnd(c.OrigSelection[1])
	}
	if c.Loc.GreaterThan(c.OrigSelection[1]) {
		forward := c.X
		lineLen := util.CharacterCount(c.buf.LineBytes(c.Y)) - 1
		for forward < lineLen && util.IsWordChar(c.RuneUnder(forward+1)) {
			forward++
		}
		c.SetSelectionEnd(Loc{forward, c.Y}.Move(1, c.buf))
		c.SetSelectionStart(c.OrigSelection[0])
	}
	c.Loc = c.CurSelection[1]
}
func (c *Cursor) SelectTo(loc Loc) {
	if loc.GreaterThan(c.OrigSelection[0]) {
		c.SetSelectionStart(c.OrigSelection[0])
		c.SetSelectionEnd(loc)
	} else {
		c.SetSelectionStart(loc)
		c.SetSelectionEnd(c.OrigSelection[0])
	}
}
func (c *Cursor) WordRight() {
	for util.IsWhitespace(c.RuneUnder(c.X)) {
		if c.X == util.CharacterCount(c.buf.LineBytes(c.Y)) {
			c.Right()
			return
		}
		c.Right()
	}
	c.Right()
	for util.IsWordChar(c.RuneUnder(c.X)) {
		if c.X == util.CharacterCount(c.buf.LineBytes(c.Y)) {
			return
		}
		c.Right()
	}
}
func (c *Cursor) WordLeft() {
	c.Left()
	for util.IsWhitespace(c.RuneUnder(c.X)) {
		if c.X == 0 {
			return
		}
		c.Left()
	}
	c.Left()
	for util.IsWordChar(c.RuneUnder(c.X)) {
		if c.X == 0 {
			return
		}
		c.Left()
	}
	c.Right()
}
func (c *Cursor) RuneUnder(x int) rune {
	line := c.buf.LineBytes(c.Y)
	if len(line) == 0 || x >= util.CharacterCount(line) {
		return '\n'
	} else if x < 0 {
		x = 0
	}
	i := 0
	for len(line) > 0 {
		r, _, size := util.DecodeCharacter(line)
		line = line[size:]
		if i == x {
			return r
		}
		i++
	}
	return '\n'
}
func (c *Cursor) StoreVisualX() {
	c.LastVisualX = c.GetVisualX()
}