package display
import (
	runewidth "github.com/mattn/go-runewidth"
	"github.com/zyedidia/micro/v2/internal/buffer"
	"github.com/zyedidia/micro/v2/internal/util"
)
type SLoc struct {
	Line, Row int
}
func (s SLoc) LessThan(b SLoc) bool {
	if s.Line < b.Line {
		return true
	}
	return s.Line == b.Line && s.Row < b.Row
}
func (s SLoc) GreaterThan(b SLoc) bool {
	if s.Line > b.Line {
		return true
	}
	return s.Line == b.Line && s.Row > b.Row
}
func (s SLoc) LessEqual(b SLoc) bool {
	if s.Line < b.Line {
		return true
	}
	if s.Line == b.Line && s.Row < b.Row {
		return true
	}
	return s == b
}
func (s SLoc) GreaterEqual(b SLoc) bool {
	if s.Line > b.Line {
		return true
	}
	if s.Line == b.Line && s.Row > b.Row {
		return true
	}
	return s == b
}
type VLoc struct {
	SLoc
	VisualX int
}
type SoftWrap interface {
	Scroll(s SLoc, n int) SLoc
	Diff(s1, s2 SLoc) int
	SLocFromLoc(loc buffer.Loc) SLoc
	VLocFromLoc(loc buffer.Loc) VLoc
	LocFromVLoc(vloc VLoc) buffer.Loc
}
func (w *BufWindow) getVLocFromLoc(loc buffer.Loc) VLoc {
	vloc := VLoc{SLoc: SLoc{loc.Y, 0}, VisualX: 0}
	if loc.X <= 0 {
		return vloc
	}
	if w.bufWidth <= 0 {
		return vloc
	}
	wordwrap := w.Buf.Settings["wordwrap"].(bool)
	tabsize := util.IntOpt(w.Buf.Settings["tabsize"])
	line := w.Buf.LineBytes(loc.Y)
	x := 0
	totalwidth := 0
	wordwidth := 0
	wordoffset := 0
	for len(line) > 0 {
		r, _, size := util.DecodeCharacter(line)
		line = line[size:]
		width := 0
		switch r {
		case '\t':
			ts := tabsize - (totalwidth % tabsize)
			width = util.Min(ts, w.bufWidth-vloc.VisualX)
			totalwidth += ts
		default:
			width = runewidth.RuneWidth(r)
			totalwidth += width
		}
		wordwidth += width
		if wordwrap {
			if !util.IsWhitespace(r) && len(line) > 0 && wordwidth < w.bufWidth {
				if x < loc.X {
					wordoffset += width
					x++
				}
				continue
			}
		}
		if vloc.VisualX+wordwidth > w.bufWidth && vloc.VisualX > 0 {
			vloc.Row++
			vloc.VisualX = 0
		}
		if x == loc.X {
			vloc.VisualX += wordoffset
			return vloc
		}
		x++
		vloc.VisualX += wordwidth
		wordwidth = 0
		wordoffset = 0
		if vloc.VisualX >= w.bufWidth {
			vloc.Row++
			vloc.VisualX = 0
		}
	}
	return vloc
}
func (w *BufWindow) getLocFromVLoc(svloc VLoc) buffer.Loc {
	loc := buffer.Loc{X: 0, Y: svloc.Line}
	if w.bufWidth <= 0 {
		return loc
	}
	wordwrap := w.Buf.Settings["wordwrap"].(bool)
	tabsize := util.IntOpt(w.Buf.Settings["tabsize"])
	line := w.Buf.LineBytes(svloc.Line)
	vloc := VLoc{SLoc: SLoc{svloc.Line, 0}, VisualX: 0}
	totalwidth := 0
	var widths []int
	if wordwrap {
		widths = make([]int, 0, w.bufWidth)
	} else {
		widths = make([]int, 0, 1)
	}
	wordwidth := 0
	for len(line) > 0 {
		r, _, size := util.DecodeCharacter(line)
		line = line[size:]
		width := 0
		switch r {
		case '\t':
			ts := tabsize - (totalwidth % tabsize)
			width = util.Min(ts, w.bufWidth-vloc.VisualX)
			totalwidth += ts
		default:
			width = runewidth.RuneWidth(r)
			totalwidth += width
		}
		widths = append(widths, width)
		wordwidth += width
		if wordwrap {
			if !util.IsWhitespace(r) && len(line) > 0 && wordwidth < w.bufWidth {
				continue
			}
		}
		if vloc.VisualX+wordwidth > w.bufWidth && vloc.VisualX > 0 {
			if vloc.Row == svloc.Row {
				if wordwrap {
					loc.X--
				}
				return loc
			}
			vloc.Row++
			vloc.VisualX = 0
		}
		for i := range widths {
			vloc.VisualX += widths[i]
			if vloc.Row == svloc.Row && vloc.VisualX > svloc.VisualX {
				return loc
			}
			loc.X++
		}
		widths = widths[:0]
		wordwidth = 0
		if vloc.VisualX >= w.bufWidth {
			vloc.Row++
			vloc.VisualX = 0
		}
	}
	return loc
}
func (w *BufWindow) getRowCount(line int) int {
	eol := buffer.Loc{X: util.CharacterCount(w.Buf.LineBytes(line)), Y: line}
	return w.getVLocFromLoc(eol).Row + 1
}
func (w *BufWindow) scrollUp(s SLoc, n int) SLoc {
	for n > 0 {
		if n <= s.Row {
			s.Row -= n
			n = 0
		} else if s.Line > 0 {
			s.Line--
			n -= s.Row + 1
			s.Row = w.getRowCount(s.Line) - 1
		} else {
			s.Row = 0
			break
		}
	}
	return s
}
func (w *BufWindow) scrollDown(s SLoc, n int) SLoc {
	for n > 0 {
		rc := w.getRowCount(s.Line)
		if n < rc-s.Row {
			s.Row += n
			n = 0
		} else if s.Line < w.Buf.LinesNum()-1 {
			s.Line++
			n -= rc - s.Row
			s.Row = 0
		} else {
			s.Row = rc - 1
			break
		}
	}
	return s
}
func (w *BufWindow) scroll(s SLoc, n int) SLoc {
	if n < 0 {
		return w.scrollUp(s, -n)
	}
	return w.scrollDown(s, n)
}
func (w *BufWindow) diff(s1, s2 SLoc) int {
	n := 0
	for s1.LessThan(s2) {
		if s1.Line < s2.Line {
			n += w.getRowCount(s1.Line) - s1.Row
			s1.Line++
			s1.Row = 0
		} else {
			n += s2.Row - s1.Row
			s1.Row = s2.Row
		}
	}
	return n
}
func (w *BufWindow) Scroll(s SLoc, n int) SLoc {
	if !w.Buf.Settings["softwrap"].(bool) {
		s.Line += n
		if s.Line < 0 {
			s.Line = 0
		}
		if s.Line > w.Buf.LinesNum()-1 {
			s.Line = w.Buf.LinesNum() - 1
		}
		return s
	}
	return w.scroll(s, n)
}
func (w *BufWindow) Diff(s1, s2 SLoc) int {
	if !w.Buf.Settings["softwrap"].(bool) {
		return s2.Line - s1.Line
	}
	if s1.GreaterThan(s2) {
		return -w.diff(s2, s1)
	}
	return w.diff(s1, s2)
}
func (w *BufWindow) SLocFromLoc(loc buffer.Loc) SLoc {
	if !w.Buf.Settings["softwrap"].(bool) {
		return SLoc{loc.Y, 0}
	}
	return w.getVLocFromLoc(loc).SLoc
}
func (w *BufWindow) VLocFromLoc(loc buffer.Loc) VLoc {
	if !w.Buf.Settings["softwrap"].(bool) {
		tabsize := util.IntOpt(w.Buf.Settings["tabsize"])
		visualx := util.StringWidth(w.Buf.LineBytes(loc.Y), loc.X, tabsize)
		return VLoc{SLoc{loc.Y, 0}, visualx}
	}
	return w.getVLocFromLoc(loc)
}
func (w *BufWindow) LocFromVLoc(vloc VLoc) buffer.Loc {
	if !w.Buf.Settings["softwrap"].(bool) {
		tabsize := util.IntOpt(w.Buf.Settings["tabsize"])
		x := util.GetCharPosInLine(w.Buf.LineBytes(vloc.Line), vloc.VisualX, tabsize)
		return buffer.Loc{x, vloc.Line}
	}
	return w.getLocFromVLoc(vloc)
}