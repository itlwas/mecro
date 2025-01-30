package display
import (
	"github.com/zyedidia/micro/v2/internal/buffer"
)
type View struct {
	X, Y          int
	Width, Height int
	StartLine SLoc
	StartCol int
}
type Window interface {
	Display()
	Clear()
	Relocate() bool
	GetView() *View
	SetView(v *View)
	LocFromVisual(vloc buffer.Loc) buffer.Loc
	Resize(w, h int)
	SetActive(b bool)
	IsActive() bool
}
type BWindow interface {
	Window
	SoftWrap
	SetBuffer(b *buffer.Buffer)
	BufView() View
}