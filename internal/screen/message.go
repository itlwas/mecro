package screen
import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)
func TermMessage(msg ...interface{}) {
	screenb := TempFini()
	fmt.Println(msg...)
	fmt.Print("\nPress enter to continue")
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
	TempStart(screenb)
}
func TermPrompt(prompt string, options []string, wait bool) int {
	screenb := TempFini()
	idx := -1
	for ok := true; ok; ok = wait && idx == -1 {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print(prompt)
		resp, _ := reader.ReadString('\n')
		resp = strings.TrimSpace(resp)
		for i, opt := range options {
			if resp == opt {
				idx = i
			}
		}
		if wait && idx == -1 {
			fmt.Println("\nInvalid choice.")
		}
	}
	TempStart(screenb)
	return idx
}
func TermError(filename string, lineNum int, err string) {
	TermMessage(filename + ", " + strconv.Itoa(lineNum) + ": " + err)
}