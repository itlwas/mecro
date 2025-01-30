package util
import (
	"unicode"
	"unicode/utf8"
)
var minMark = rune(unicode.Mark.R16[0].Lo)
func isMark(r rune) bool {
	if r < minMark {
		return false
	}
	return unicode.In(r, unicode.Mark)
}
func DecodeCharacter(b []byte) (rune, []rune, int) {
	r, size := utf8.DecodeRune(b)
	b = b[size:]
	c, s := utf8.DecodeRune(b)
	var combc []rune
	for isMark(c) {
		combc = append(combc, c)
		size += s
		b = b[s:]
		c, s = utf8.DecodeRune(b)
	}
	return r, combc, size
}
func DecodeCharacterInString(str string) (rune, []rune, int) {
	r, size := utf8.DecodeRuneInString(str)
	str = str[size:]
	c, s := utf8.DecodeRuneInString(str)
	var combc []rune
	for isMark(c) {
		combc = append(combc, c)
		size += s
		str = str[s:]
		c, s = utf8.DecodeRuneInString(str)
	}
	return r, combc, size
}
func CharacterCount(b []byte) int {
	s := 0
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if !isMark(r) {
			s++
		}
		b = b[size:]
	}
	return s
}
func CharacterCountInString(str string) int {
	s := 0
	for _, r := range str {
		if !isMark(r) {
			s++
		}
	}
	return s
}