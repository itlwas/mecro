package clipboard
import (
	"errors"
	"github.com/zyedidia/clipper"
)
type Method int
const (
	External Method = iota
	Terminal
	Internal
)
var CurrentMethod Method = Internal
type Register int
const (
	ClipboardReg Register = -1
	PrimaryReg = -2
)
var clipboard clipper.Clipboard
func Initialize(m Method) error {
	var err error
	switch m {
	case External:
		clips := make([]clipper.Clipboard, 0, len(clipper.Clipboards)+1)
		clips = append(clips, &clipper.Custom{
			Name: "mecro-clip",
		})
		clips = append(clips, clipper.Clipboards...)
		clipboard, err = clipper.GetClipboard(clips...)
	}
	if err != nil {
		CurrentMethod = Internal
	}
	return err
}
func SetMethod(m string) Method {
	switch m {
	case "internal":
		CurrentMethod = Internal
	case "external":
		CurrentMethod = External
	case "terminal":
		CurrentMethod = Terminal
	}
	return CurrentMethod
}
func Read(r Register) (string, error) {
	return read(r, CurrentMethod)
}
func Write(text string, r Register) error {
	return write(text, r, CurrentMethod)
}
func ReadMulti(r Register, num, ncursors int) (string, error) {
	clip, err := Read(r)
	if err != nil {
		return "", err
	}
	if ValidMulti(r, clip, ncursors) {
		return multi.getText(r, num), nil
	}
	return clip, nil
}
func WriteMulti(text string, r Register, num int, ncursors int) error {
	return writeMulti(text, r, num, ncursors, CurrentMethod)
}
func ValidMulti(r Register, clip string, ncursors int) bool {
	return multi.isValid(r, clip, ncursors)
}
func writeMulti(text string, r Register, num int, ncursors int, m Method) error {
	multi.writeText(text, r, num, ncursors)
	return write(multi.getAllText(r), r, m)
}
func read(r Register, m Method) (string, error) {
	switch m {
	case External:
		switch r {
		case ClipboardReg:
			b, e := clipboard.ReadAll(clipper.RegClipboard)
			return string(b), e
		case PrimaryReg:
			b, e := clipboard.ReadAll(clipper.RegPrimary)
			return string(b), e
		default:
			return internal.read(r), nil
		}
	case Internal:
		return internal.read(r), nil
	case Terminal:
		switch r {
		case ClipboardReg:
			return terminal.read("clipboard")
		case PrimaryReg:
			return terminal.read("primary")
		default:
			return internal.read(r), nil
		}
	}
	return "", errors.New("Invalid clipboard method")
}
func write(text string, r Register, m Method) error {
	switch m {
	case External:
		switch r {
		case ClipboardReg:
			return clipboard.WriteAll(clipper.RegClipboard, []byte(text))
		case PrimaryReg:
			return clipboard.WriteAll(clipper.RegPrimary, []byte(text))
		default:
			internal.write(text, r)
		}
	case Internal:
		internal.write(text, r)
	case Terminal:
		switch r {
		case ClipboardReg:
			return terminal.write(text, "c")
		case PrimaryReg:
			return terminal.write(text, "p")
		default:
			internal.write(text, r)
		}
	}
	return nil
}