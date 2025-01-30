package buffer
type TEStack struct {
	Top  *Element
	Size int
}
type Element struct {
	Value *TextEvent
	Next  *Element
}
func (s *TEStack) Len() int {
	return s.Size
}
func (s *TEStack) Push(value *TextEvent) {
	s.Top = &Element{value, s.Top}
	s.Size++
}
func (s *TEStack) Pop() (value *TextEvent) {
	if s.Size > 0 {
		value, s.Top = s.Top.Value, s.Top.Next
		s.Size--
		return
	}
	return nil
}
func (s *TEStack) Peek() *TextEvent {
	if s.Size > 0 {
		return s.Top.Value
	}
	return nil
}