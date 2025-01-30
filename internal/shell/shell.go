package shell
import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	shellquote "github.com/kballard/go-shellquote"
	"github.com/zyedidia/micro/v2/internal/screen"
)
func ExecCommand(name string, arg ...string) (string, error) {
	var err error
	cmd := exec.Command(name, arg...)
	outputBytes := &bytes.Buffer{}
	cmd.Stdout = outputBytes
	cmd.Stderr = outputBytes
	err = cmd.Start()
	if err != nil {
		return "", err
	}
	err = cmd.Wait()
	outstring := outputBytes.String()
	return outstring, err
}
func RunCommand(input string) (string, error) {
	args, err := shellquote.Split(input)
	if err != nil {
		return "", err
	}
	if len(args) == 0 {
		return "", errors.New("No arguments")
	}
	inputCmd := args[0]
	return ExecCommand(inputCmd, args[1:]...)
}
func RunBackgroundShell(input string) (func() string, error) {
	args, err := shellquote.Split(input)
	if err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, errors.New("No arguments")
	}
	inputCmd := args[0]
	return func() string {
		output, err := RunCommand(input)
		str := output
		if err != nil {
			str = fmt.Sprint(inputCmd, " exited with error: ", err, ": ", output)
		}
		return str
	}, nil
}
func RunInteractiveShell(input string, wait bool, getOutput bool) (string, error) {
	args, err := shellquote.Split(input)
	if err != nil {
		return "", err
	}
	if len(args) == 0 {
		return "", errors.New("No arguments")
	}
	inputCmd := args[0]
	screenb := screen.TempFini()
	args = args[1:]
	outputBytes := &bytes.Buffer{}
	cmd := exec.Command(inputCmd, args...)
	cmd.Stdin = os.Stdin
	if getOutput {
		cmd.Stdout = io.MultiWriter(os.Stdout, outputBytes)
	} else {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = os.Stderr
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			cmd.Process.Kill()
		}
	}()
	cmd.Start()
	err = cmd.Wait()
	output := outputBytes.String()
	if wait {
		screen.TermMessage("")
	}
	screen.TempStart(screenb)
	return output, err
}