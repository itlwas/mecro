package action
import (
	"bytes"
	"github.com/zyedidia/tcell/v2"
)
type PaneKeyAction func(Pane) bool
type PaneMouseAction func(Pane, *tcell.EventMouse) bool
type PaneKeyAnyAction func(Pane, []KeyEvent) bool
type KeyTreeNode struct {
	children map[Event]*KeyTreeNode
	actions []TreeAction
}
func NewKeyTreeNode() *KeyTreeNode {
	n := new(KeyTreeNode)
	n.children = make(map[Event]*KeyTreeNode)
	n.actions = []TreeAction{}
	return n
}
type TreeAction struct {
	action PaneKeyAction
	any    PaneKeyAnyAction
	mouse  PaneMouseAction
	modes []ModeConstraint
}
type KeyTree struct {
	root  *KeyTreeNode
	modes map[string]bool
	cursor KeyTreeCursor
}
type KeyTreeCursor struct {
	node *KeyTreeNode
	recordedEvents []Event
	wildcards      []KeyEvent
	mouseInfo      *tcell.EventMouse
}
func (k *KeyTreeCursor) MakeClosure(a TreeAction) PaneKeyAction {
	if a.action != nil {
		return a.action
	} else if a.any != nil {
		return func(p Pane) bool {
			return a.any(p, k.wildcards)
		}
	} else if a.mouse != nil {
		return func(p Pane) bool {
			return a.mouse(p, k.mouseInfo)
		}
	}
	return nil
}
func NewKeyTree() *KeyTree {
	root := NewKeyTreeNode()
	tree := new(KeyTree)
	tree.root = root
	tree.modes = make(map[string]bool)
	tree.cursor = KeyTreeCursor{
		node:      root,
		wildcards: []KeyEvent{},
		mouseInfo: nil,
	}
	return tree
}
type ModeConstraint struct {
	mode     string
	disabled bool
}
func (k *KeyTree) RegisterKeyBinding(e Event, a PaneKeyAction) {
	k.registerBinding(e, TreeAction{
		action: a,
		any:    nil,
		mouse:  nil,
		modes:  nil,
	})
}
func (k *KeyTree) RegisterKeyAnyBinding(e Event, a PaneKeyAnyAction) {
	k.registerBinding(e, TreeAction{
		action: nil,
		any:    a,
		mouse:  nil,
		modes:  nil,
	})
}
func (k *KeyTree) RegisterMouseBinding(e Event, a PaneMouseAction) {
	k.registerBinding(e, TreeAction{
		action: nil,
		any:    nil,
		mouse:  a,
		modes:  nil,
	})
}
func (k *KeyTree) registerBinding(e Event, a TreeAction) {
	switch ev := e.(type) {
	case KeyEvent, MouseEvent, RawEvent:
		newNode, ok := k.root.children[e]
		if !ok {
			newNode = NewKeyTreeNode()
			k.root.children[e] = newNode
		}
		newNode.actions = []TreeAction{a}
	case KeySequenceEvent:
		n := k.root
		for _, key := range ev.keys {
			newNode, ok := n.children[key]
			if !ok {
				newNode = NewKeyTreeNode()
				n.children[key] = newNode
			}
			n = newNode
		}
		n.actions = []TreeAction{a}
	}
}
func (k *KeyTree) NextEvent(e Event, mouse *tcell.EventMouse) (PaneKeyAction, bool) {
	n := k.cursor.node
	c, ok := n.children[e]
	if !ok {
		return nil, false
	}
	more := len(c.children) > 0
	k.cursor.node = c
	k.cursor.recordedEvents = append(k.cursor.recordedEvents, e)
	switch ev := e.(type) {
	case KeyEvent:
		if ev.any {
			k.cursor.wildcards = append(k.cursor.wildcards, ev)
		}
	case MouseEvent:
		k.cursor.mouseInfo = mouse
	}
	if len(c.actions) > 0 {
		for _, a := range c.actions {
			active := true
			for _, mc := range a.modes {
				hasMode := k.modes[mc.mode]
				if hasMode != mc.disabled {
					active = false
				}
			}
			if active {
				return k.cursor.MakeClosure(a), more
			}
		}
	}
	return nil, more
}
func (k *KeyTree) ResetEvents() {
	k.cursor.node = k.root
	k.cursor.wildcards = []KeyEvent{}
	k.cursor.recordedEvents = []Event{}
	k.cursor.mouseInfo = nil
}
func (k *KeyTree) RecordedEventsStr() string {
	buf := &bytes.Buffer{}
	for _, e := range k.cursor.recordedEvents {
		buf.WriteString(e.Name())
	}
	return buf.String()
}
func (k *KeyTree) DeleteBinding(e Event) {
}
func (k *KeyTree) DeleteAllBindings(e Event) {
}
func (k *KeyTree) SetMode(mode string, en bool) {
	k.modes[mode] = en
}
func (k *KeyTree) HasMode(mode string) bool {
	return k.modes[mode]
}