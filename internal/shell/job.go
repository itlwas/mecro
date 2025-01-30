package shell
import (
	"bytes"
	"io"
	"os/exec"
)
var Jobs chan JobFunction
func init() {
	Jobs = make(chan JobFunction, 100)
}
type JobFunction struct {
	Function func(string, []interface{})
	Output   string
	Args     []interface{}
}
type CallbackFile struct {
	io.Writer
	callback func(string, []interface{})
	args     []interface{}
}
type Job struct {
	*exec.Cmd
	Stdin io.WriteCloser
}
func (f *CallbackFile) Write(data []byte) (int, error) {
	jobFunc := JobFunction{f.callback, string(data), f.args}
	Jobs <- jobFunc
	return f.Writer.Write(data)
}
func JobStart(cmd string, onStdout, onStderr, onExit func(string, []interface{}), userargs ...interface{}) *Job {
	return JobSpawn("sh", []string{"-c", cmd}, onStdout, onStderr, onExit, userargs...)
}
func JobSpawn(cmdName string, cmdArgs []string, onStdout, onStderr, onExit func(string, []interface{}), userargs ...interface{}) *Job {
	proc := exec.Command(cmdName, cmdArgs...)
	var outbuf bytes.Buffer
	if onStdout != nil {
		proc.Stdout = &CallbackFile{&outbuf, onStdout, userargs}
	} else {
		proc.Stdout = &outbuf
	}
	if onStderr != nil {
		proc.Stderr = &CallbackFile{&outbuf, onStderr, userargs}
	} else {
		proc.Stderr = &outbuf
	}
	stdin, _ := proc.StdinPipe()
	go func() {
		proc.Run()
		jobFunc := JobFunction{onExit, outbuf.String(), userargs}
		Jobs <- jobFunc
	}()
	return &Job{proc, stdin}
}
func JobStop(j *Job) {
	j.Process.Kill()
}
func JobSend(j *Job, data string) {
	j.Stdin.Write([]byte(data))
}