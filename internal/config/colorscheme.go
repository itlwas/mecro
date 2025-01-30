package config
import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"github.com/zyedidia/tcell/v2"
)
var DefStyle tcell.Style = tcell.StyleDefault
var Colorscheme map[string]tcell.Style
func GetColor(color string) tcell.Style {
	st := DefStyle
	if color == "" {
		return st
	}
	groups := strings.Split(color, ".")
	if len(groups) > 1 {
		curGroup := ""
		for i, g := range groups {
			if i != 0 {
				curGroup += "."
			}
			curGroup += g
			if style, ok := Colorscheme[curGroup]; ok {
				st = style
			}
		}
	} else if style, ok := Colorscheme[color]; ok {
		st = style
	} else {
		st = StringToStyle(color)
	}
	return st
}
func ColorschemeExists(colorschemeName string) bool {
	return FindRuntimeFile(RTColorscheme, colorschemeName) != nil
}
func InitColorscheme() error {
	Colorscheme = make(map[string]tcell.Style)
	DefStyle = tcell.StyleDefault
	c, err := LoadDefaultColorscheme()
	if err == nil {
		Colorscheme = c
	}
	return err
}
func LoadDefaultColorscheme() (map[string]tcell.Style, error) {
	var parsedColorschemes []string
	return LoadColorscheme(GlobalSettings["colorscheme"].(string), &parsedColorschemes)
}
func LoadColorscheme(colorschemeName string, parsedColorschemes *[]string) (map[string]tcell.Style, error) {
	c := make(map[string]tcell.Style)
	file := FindRuntimeFile(RTColorscheme, colorschemeName)
	if file == nil {
		return c, errors.New(colorschemeName + " is not a valid colorscheme")
	}
	if data, err := file.Data(); err != nil {
		return c, errors.New("Error loading colorscheme: " + err.Error())
	} else {
		var err error
		c, err = ParseColorscheme(file.Name(), string(data), parsedColorschemes)
		if err != nil {
			return c, err
		}
	}
	return c, nil
}
func ParseColorscheme(name string, text string, parsedColorschemes *[]string) (map[string]tcell.Style, error) {
	var err error
	colorParser := regexp.MustCompile(`color-link\s+(\S*)\s+"(.*)"`)
	includeParser := regexp.MustCompile(`include\s+"(.*)"`)
	lines := strings.Split(text, "\n")
	c := make(map[string]tcell.Style)
	if parsedColorschemes != nil {
		*parsedColorschemes = append(*parsedColorschemes, name)
	}
lineLoop:
	for _, line := range lines {
		if strings.TrimSpace(line) == "" ||
			strings.TrimSpace(line)[0] == '#' {
			continue
		}
		matches := includeParser.FindSubmatch([]byte(line))
		if len(matches) == 2 {
			if parsedColorschemes != nil {
				include := string(matches[1])
				for _, name := range *parsedColorschemes {
					if name == include {
						continue lineLoop
					}
				}
				includeScheme, err := LoadColorscheme(include, parsedColorschemes)
				if err != nil {
					return c, err
				}
				for k, v := range includeScheme {
					c[k] = v
				}
			}
			continue
		}
		matches = colorParser.FindSubmatch([]byte(line))
		if len(matches) == 3 {
			link := string(matches[1])
			colors := string(matches[2])
			style := StringToStyle(colors)
			c[link] = style
			if link == "default" {
				DefStyle = style
			}
		} else {
			err = errors.New("Color-link statement is not valid: " + line)
		}
	}
	return c, err
}
func StringToStyle(str string) tcell.Style {
	var fg, bg string
	spaceSplit := strings.Split(str, " ")
	split := strings.Split(spaceSplit[len(spaceSplit)-1], ",")
	if len(split) > 1 {
		fg, bg = split[0], split[1]
	} else {
		fg = split[0]
	}
	fg = strings.TrimSpace(fg)
	bg = strings.TrimSpace(bg)
	var fgColor, bgColor tcell.Color
	var ok bool
	if fg == "" || fg == "default" {
		fgColor, _, _ = DefStyle.Decompose()
	} else {
		fgColor, ok = StringToColor(fg)
		if !ok {
			fgColor, _, _ = DefStyle.Decompose()
		}
	}
	if bg == "" || bg == "default" {
		_, bgColor, _ = DefStyle.Decompose()
	} else {
		bgColor, ok = StringToColor(bg)
		if !ok {
			_, bgColor, _ = DefStyle.Decompose()
		}
	}
	style := DefStyle.Foreground(fgColor).Background(bgColor)
	if strings.Contains(str, "bold") {
		style = style.Bold(true)
	}
	if strings.Contains(str, "italic") {
		style = style.Italic(true)
	}
	if strings.Contains(str, "reverse") {
		style = style.Reverse(true)
	}
	if strings.Contains(str, "underline") {
		style = style.Underline(true)
	}
	return style
}
func StringToColor(str string) (tcell.Color, bool) {
	switch str {
	case "black":
		return tcell.ColorBlack, true
	case "red":
		return tcell.ColorMaroon, true
	case "green":
		return tcell.ColorGreen, true
	case "yellow":
		return tcell.ColorOlive, true
	case "blue":
		return tcell.ColorNavy, true
	case "magenta":
		return tcell.ColorPurple, true
	case "cyan":
		return tcell.ColorTeal, true
	case "white":
		return tcell.ColorSilver, true
	case "brightblack", "lightblack":
		return tcell.ColorGray, true
	case "brightred", "lightred":
		return tcell.ColorRed, true
	case "brightgreen", "lightgreen":
		return tcell.ColorLime, true
	case "brightyellow", "lightyellow":
		return tcell.ColorYellow, true
	case "brightblue", "lightblue":
		return tcell.ColorBlue, true
	case "brightmagenta", "lightmagenta":
		return tcell.ColorFuchsia, true
	case "brightcyan", "lightcyan":
		return tcell.ColorAqua, true
	case "brightwhite", "lightwhite":
		return tcell.ColorWhite, true
	case "default":
		return tcell.ColorDefault, true
	default:
		if num, err := strconv.Atoi(str); err == nil {
			return GetColor256(num), true
		}
		if len(str) == 7 && str[0] == '#' {
			return tcell.GetColor(str), true
		}
		return tcell.ColorDefault, false
	}
}
func GetColor256(color int) tcell.Color {
	if color == 0 {
		return tcell.ColorDefault
	}
	return tcell.PaletteColor(color)
}