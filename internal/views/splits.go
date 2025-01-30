package views
import (
	"fmt"
	"strings"
)
type SplitType uint8
const (
	STVert  = 0
	STHoriz = 1
	STUndef = 2
)
var idcounter uint64
func NewID() uint64 {
	idcounter++
	return idcounter
}
type View struct {
	X, Y int
	W, H int
}
type Node struct {
	View
	Kind SplitType
	parent   *Node
	children []*Node
	canResize bool
	propScale bool
	propW, propH float64
	id uint64
}
func NewNode(Kind SplitType, x, y, w, h int, parent *Node, id uint64) *Node {
	n := new(Node)
	n.Kind = Kind
	n.canResize = true
	n.propScale = true
	n.X, n.Y, n.W, n.H = x, y, w, h
	n.children = make([]*Node, 0)
	n.parent = parent
	n.id = id
	if parent != nil {
		n.propW, n.propH = float64(w)/float64(parent.W), float64(h)/float64(parent.H)
	} else {
		n.propW, n.propH = 1, 1
	}
	return n
}
func NewRoot(x, y, w, h int) *Node {
	n1 := NewNode(STUndef, x, y, w, h, nil, NewID())
	return n1
}
func (n *Node) IsLeaf() bool {
	return len(n.children) == 0
}
func (n *Node) ID() uint64 {
	if n.IsLeaf() {
		return n.id
	}
	return 0
}
func (n *Node) CanResize() bool {
	return n.canResize
}
func (n *Node) PropScale() bool {
	return n.propScale
}
func (n *Node) SetResize(b bool) {
	n.canResize = b
}
func (n *Node) SetPropScale(b bool) {
	n.propScale = b
}
func (n *Node) Children() []*Node {
	return n.children
}
func (n *Node) GetNode(id uint64) *Node {
	if n.id == id && n.IsLeaf() {
		return n
	}
	for _, c := range n.children {
		if c.id == id && c.IsLeaf() {
			return c
		}
		gc := c.GetNode(id)
		if gc != nil {
			return gc
		}
	}
	return nil
}
func (n *Node) vResizeSplit(i int, size int) bool {
	if i < 0 || i >= len(n.children) {
		return false
	}
	var c1, c2 *Node
	if i == len(n.children)-1 {
		c1, c2 = n.children[i-1], n.children[i]
	} else {
		c1, c2 = n.children[i], n.children[i+1]
	}
	toth := c1.H + c2.H
	if size >= toth {
		return false
	}
	c2.Y = c1.Y + size
	c1.Resize(c1.W, size)
	c2.Resize(c2.W, toth-size)
	n.markSizes()
	n.alignSizes(n.W, n.H)
	return true
}
func (n *Node) hResizeSplit(i int, size int) bool {
	if i < 0 || i >= len(n.children) {
		return false
	}
	var c1, c2 *Node
	if i == len(n.children)-1 {
		c1, c2 = n.children[i-1], n.children[i]
	} else {
		c1, c2 = n.children[i], n.children[i+1]
	}
	totw := c1.W + c2.W
	if size >= totw {
		return false
	}
	c2.X = c1.X + size
	c1.Resize(size, c1.H)
	c2.Resize(totw-size, c2.H)
	n.markSizes()
	n.alignSizes(n.W, n.H)
	return true
}
func (n *Node) ResizeSplit(size int) bool {
	if size <= 0 {
		return false
	}
	if len(n.parent.children) <= 1 {
		return false
	}
	ind := 0
	for i, c := range n.parent.children {
		if c.id == n.id {
			ind = i
		}
	}
	if n.parent.Kind == STVert {
		return n.parent.vResizeSplit(ind, size)
	}
	return n.parent.hResizeSplit(ind, size)
}
func (n *Node) Resize(w, h int) {
	n.W, n.H = w, h
	if n.IsLeaf() {
		return
	}
	x, y := n.X, n.Y
	totw, toth := 0, 0
	for _, c := range n.children {
		cW := int(float64(w) * c.propW)
		cH := int(float64(h) * c.propH)
		c.X, c.Y = x, y
		c.Resize(cW, cH)
		if n.Kind == STHoriz {
			x += cW
			totw += cW
		} else {
			y += cH
			toth += cH
		}
	}
	n.alignSizes(totw, toth)
}
func (n *Node) alignSizes(totw, toth int) {
	if n.Kind == STVert && toth != n.H {
		last := n.children[len(n.children)-1]
		last.Resize(last.W, last.H+n.H-toth)
	} else if n.Kind == STHoriz && totw != n.W {
		last := n.children[len(n.children)-1]
		last.Resize(last.W+n.W-totw, last.H)
	}
}
func (n *Node) markSizes() {
	for _, c := range n.children {
		c.propW = float64(c.W) / float64(n.W)
		c.propH = float64(c.H) / float64(n.H)
		c.markSizes()
	}
}
func (n *Node) markResize() {
	n.markSizes()
	n.Resize(n.W, n.H)
}
func (n *Node) vVSplit(right bool) uint64 {
	ind := 0
	for i, c := range n.parent.children {
		if c.id == n.id {
			ind = i
		}
	}
	return n.parent.hVSplit(ind, right)
}
func (n *Node) hHSplit(bottom bool) uint64 {
	ind := 0
	for i, c := range n.parent.children {
		if c.id == n.id {
			ind = i
		}
	}
	return n.parent.vHSplit(ind, bottom)
}
func (n *Node) getResizeInfo(h bool) (int, int) {
	numr := 0
	numnr := 0
	nonr := 0
	for _, c := range n.children {
		if !c.CanResize() {
			if h {
				nonr += c.H
			} else {
				nonr += c.W
			}
			numnr++
		} else {
			numr++
		}
	}
	if numr == 0 {
		numr = numnr
	}
	return nonr, numr
}
func (n *Node) applyNewSize(size int, h bool) {
	a := n.X
	if h {
		a = n.Y
	}
	for _, c := range n.children {
		if h {
			c.Y = a
		} else {
			c.X = a
		}
		if c.CanResize() {
			if h {
				c.Resize(c.W, size)
			} else {
				c.Resize(size, c.H)
			}
		}
		if h {
			a += c.H
		} else {
			a += c.H
		}
	}
	n.markResize()
}
func (n *Node) vHSplit(i int, right bool) uint64 {
	if n.IsLeaf() {
		newid := NewID()
		hn1 := NewNode(STHoriz, n.X, n.Y, n.W, n.H/2, n, n.id)
		hn2 := NewNode(STHoriz, n.X, n.Y+hn1.H, n.W, n.H/2, n, newid)
		if !right {
			hn1.id, hn2.id = hn2.id, hn1.id
		}
		n.children = append(n.children, hn1, hn2)
		n.markResize()
		return newid
	} else {
		nonrh, numr := n.getResizeInfo(true)
		height := (n.H - nonrh) / (numr + 1)
		newid := NewID()
		hn := NewNode(STHoriz, n.X, 0, n.W, height, n, newid)
		n.children = append(n.children, nil)
		inspos := i
		if right {
			inspos++
		}
		copy(n.children[inspos+1:], n.children[inspos:])
		n.children[inspos] = hn
		n.applyNewSize(height, true)
		return newid
	}
}
func (n *Node) hVSplit(i int, right bool) uint64 {
	if n.IsLeaf() {
		newid := NewID()
		vn1 := NewNode(STVert, n.X, n.Y, n.W/2, n.H, n, n.id)
		vn2 := NewNode(STVert, n.X+vn1.W, n.Y, n.W/2, n.H, n, newid)
		if !right {
			vn1.id, vn2.id = vn2.id, vn1.id
		}
		n.children = append(n.children, vn1, vn2)
		n.markResize()
		return newid
	} else {
		nonrw, numr := n.getResizeInfo(false)
		width := (n.W - nonrw) / (numr + 1)
		newid := NewID()
		vn := NewNode(STVert, 0, n.Y, width, n.H, n, newid)
		n.children = append(n.children, nil)
		inspos := i
		if right {
			inspos++
		}
		copy(n.children[inspos+1:], n.children[inspos:])
		n.children[inspos] = vn
		n.applyNewSize(width, false)
		return newid
	}
}
func (n *Node) HSplit(bottom bool) uint64 {
	if !n.IsLeaf() {
		return 0
	}
	if n.Kind == STUndef {
		n.Kind = STVert
	}
	if n.Kind == STVert {
		return n.vHSplit(0, bottom)
	}
	return n.hHSplit(bottom)
}
func (n *Node) VSplit(right bool) uint64 {
	if !n.IsLeaf() {
		return 0
	}
	if n.Kind == STUndef {
		n.Kind = STHoriz
	}
	if n.Kind == STVert {
		return n.vVSplit(right)
	}
	return n.hVSplit(0, right)
}
func (n *Node) unsplit(i int, h bool) {
	copy(n.children[i:], n.children[i+1:])
	n.children[len(n.children)-1] = nil
	n.children = n.children[:len(n.children)-1]
	nonrs, numr := n.getResizeInfo(h)
	if numr == 0 {
		return
	}
	size := (n.W - nonrs) / numr
	if h {
		size = (n.H - nonrs) / numr
	}
	n.applyNewSize(size, h)
}
func (n *Node) Unsplit() bool {
	if !n.IsLeaf() || n.parent == nil {
		return false
	}
	ind := 0
	for i, c := range n.parent.children {
		if c.id == n.id {
			ind = i
		}
	}
	if n.parent.Kind == STVert {
		n.parent.unsplit(ind, true)
	} else {
		n.parent.unsplit(ind, false)
	}
	if n.parent.IsLeaf() {
		return n.parent.Unsplit()
	}
	return true
}
func (n *Node) String() string {
	var strf func(n *Node, ident int) string
	strf = func(n *Node, ident int) string {
		marker := "|"
		if n.Kind == STHoriz {
			marker = "-"
		}
		str := fmt.Sprint(strings.Repeat("\t", ident), marker, n.View, n.id)
		if n.IsLeaf() {
			str += "🍁"
		}
		str += "\n"
		for _, c := range n.children {
			str += strf(c, ident+1)
		}
		return str
	}
	return strf(n, 0)
}