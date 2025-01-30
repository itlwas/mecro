package action
import (
	luar "layeh.com/gopher-luar"
	"github.com/zyedidia/micro/v2/internal/buffer"
	"github.com/zyedidia/micro/v2/internal/config"
	"github.com/zyedidia/micro/v2/internal/display"
	ulua "github.com/zyedidia/micro/v2/internal/lua"
	"github.com/zyedidia/micro/v2/internal/screen"
	"github.com/zyedidia/micro/v2/internal/views"
	"github.com/zyedidia/tcell/v2"
)
type TabList struct {
	*display.TabWindow
	List []*Tab
}
func NewTabList(bufs []*buffer.Buffer) *TabList {
	w, h := screen.Screen.Size()
	iOffset := config.GetInfoBarOffset()
	tl := new(TabList)
	tl.List = make([]*Tab, len(bufs))
	if len(bufs) > 1 {
		for i, b := range bufs {
			tl.List[i] = NewTabFromBuffer(0, 1, w, h-1-iOffset, b)
		}
	} else {
		tl.List[0] = NewTabFromBuffer(0, 0, w, h-iOffset, bufs[0])
	}
	tl.TabWindow = display.NewTabWindow(w, 0)
	tl.Names = make([]string, len(bufs))
	return tl
}
func (t *TabList) UpdateNames() {
	t.Names = t.Names[:0]
	for _, p := range t.List {
		t.Names = append(t.Names, p.Panes[p.active].Name())
	}
}
func (t *TabList) AddTab(p *Tab) {
	t.List = append(t.List, p)
	t.Resize()
	t.UpdateNames()
}
func (t *TabList) RemoveTab(id uint64) {
	for i, p := range t.List {
		if len(p.Panes) == 0 {
			continue
		}
		if p.Panes[0].ID() == id {
			copy(t.List[i:], t.List[i+1:])
			t.List[len(t.List)-1] = nil
			t.List = t.List[:len(t.List)-1]
			if t.Active() >= len(t.List) {
				t.SetActive(len(t.List) - 1)
			}
			t.Resize()
			t.UpdateNames()
			return
		}
	}
}
func (t *TabList) Resize() {
	w, h := screen.Screen.Size()
	iOffset := config.GetInfoBarOffset()
	InfoBar.Resize(w, h-1)
	if len(t.List) > 1 {
		for _, p := range t.List {
			p.Y = 1
			p.Node.Resize(w, h-1-iOffset)
			p.Resize()
		}
	} else if len(t.List) == 1 {
		t.List[0].Y = 0
		t.List[0].Node.Resize(w, h-iOffset)
		t.List[0].Resize()
	}
	t.TabWindow.Resize(w, h)
}
func (t *TabList) HandleEvent(event tcell.Event) {
	switch e := event.(type) {
	case *tcell.EventResize:
		t.Resize()
	case *tcell.EventMouse:
		mx, my := e.Position()
		switch e.Buttons() {
		case tcell.Button1:
			if my == t.Y && mx == 0 {
				t.Scroll(-4)
				return
			} else if my == t.Y && mx == t.Width-1 {
				t.Scroll(4)
				return
			}
			if len(t.List) > 1 {
				ind := t.LocFromVisual(buffer.Loc{mx, my})
				if ind != -1 {
					t.SetActive(ind)
					return
				}
				if my == 0 {
					return
				}
			}
		case tcell.WheelUp:
			if my == t.Y {
				t.Scroll(4)
				return
			}
		case tcell.WheelDown:
			if my == t.Y {
				t.Scroll(-4)
				return
			}
		}
	}
	t.List[t.Active()].HandleEvent(event)
}
func (t *TabList) Display() {
	t.UpdateNames()
	if len(t.List) > 1 {
		t.TabWindow.Display()
	}
}
var Tabs *TabList
func InitTabs(bufs []*buffer.Buffer) {
	multiopen := config.GetGlobalOption("multiopen").(string)
	if multiopen == "tab" {
		Tabs = NewTabList(bufs)
	} else {
		Tabs = NewTabList(bufs[:1])
		for _, b := range bufs[1:] {
			if multiopen == "vsplit" {
				MainTab().CurPane().VSplitBuf(b)
			} else {
				MainTab().CurPane().HSplitBuf(b)
			}
		}
	}
	screen.RestartCallback = func() {
		for _, t := range Tabs.List {
			t.release = true
			for _, p := range t.Panes {
				if bp, ok := p.(*BufPane); ok {
					bp.resetMouse()
				}
			}
		}
	}
}
func MainTab() *Tab {
	return Tabs.List[Tabs.Active()]
}
type Tab struct {
	*views.Node
	*display.UIWindow
	Panes  []Pane
	active int
	resizing *views.Node
	release bool
}
func NewTabFromBuffer(x, y, width, height int, b *buffer.Buffer) *Tab {
	t := new(Tab)
	t.Node = views.NewRoot(x, y, width, height)
	t.UIWindow = display.NewUIWindow(t.Node)
	t.release = true
	e := NewBufPaneFromBuf(b, t)
	e.SetID(t.ID())
	t.Panes = append(t.Panes, e)
	return t
}
func NewTabFromPane(x, y, width, height int, pane Pane) *Tab {
	t := new(Tab)
	t.Node = views.NewRoot(x, y, width, height)
	t.UIWindow = display.NewUIWindow(t.Node)
	t.release = true
	pane.SetTab(t)
	pane.SetID(t.ID())
	t.Panes = append(t.Panes, pane)
	return t
}
func (t *Tab) HandleEvent(event tcell.Event) {
	switch e := event.(type) {
	case *tcell.EventMouse:
		mx, my := e.Position()
		btn := e.Buttons()
		switch {
		case btn & ^(tcell.WheelUp|tcell.WheelDown|tcell.WheelLeft|tcell.WheelRight) != tcell.ButtonNone:
			wasReleased := t.release
			t.release = false
			if btn == tcell.Button1 {
				if t.resizing != nil {
					var size int
					if t.resizing.Kind == views.STVert {
						size = mx - t.resizing.X
					} else {
						size = my - t.resizing.Y + 1
					}
					t.resizing.ResizeSplit(size)
					t.Resize()
					return
				}
				if wasReleased {
					t.resizing = t.GetMouseSplitNode(buffer.Loc{mx, my})
					if t.resizing != nil {
						return
					}
				}
			}
			if wasReleased {
				for i, p := range t.Panes {
					v := p.GetView()
					inpane := mx >= v.X && mx < v.X+v.Width && my >= v.Y && my < v.Y+v.Height
					if inpane {
						t.SetActive(i)
						break
					}
				}
			}
		case btn == tcell.ButtonNone:
			t.release = true
			if t.resizing != nil {
				t.resizing = nil
				return
			}
		default:
			for _, p := range t.Panes {
				v := p.GetView()
				inpane := mx >= v.X && mx < v.X+v.Width && my >= v.Y && my < v.Y+v.Height
				if inpane {
					p.HandleEvent(event)
					return
				}
			}
		}
	}
	t.Panes[t.active].HandleEvent(event)
}
func (t *Tab) SetActive(i int) {
	t.active = i
	for j, p := range t.Panes {
		if j == i {
			p.SetActive(true)
		} else {
			p.SetActive(false)
		}
	}
	err := config.RunPluginFn("onSetActive", luar.New(ulua.L, MainTab().CurPane()))
	if err != nil {
		screen.TermMessage(err)
	}
}
func (t *Tab) GetPane(splitid uint64) int {
	for i, p := range t.Panes {
		if p.ID() == splitid {
			return i
		}
	}
	return 0
}
func (t *Tab) RemovePane(i int) {
	copy(t.Panes[i:], t.Panes[i+1:])
	t.Panes[len(t.Panes)-1] = nil
	t.Panes = t.Panes[:len(t.Panes)-1]
}
func (t *Tab) Resize() {
	for _, p := range t.Panes {
		n := t.GetNode(p.ID())
		pv := p.GetView()
		offset := 0
		if n.X != 0 {
			offset = 1
		}
		pv.X, pv.Y = n.X+offset, n.Y
		p.SetView(pv)
		p.Resize(n.W-offset, n.H)
	}
}
func (t *Tab) CurPane() *BufPane {
	p, ok := t.Panes[t.active].(*BufPane)
	if !ok {
		return nil
	}
	return p
}