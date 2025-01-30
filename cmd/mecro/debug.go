package main
import (
	"log"
	"os"
	"github.com/zyedidia/micro/v2/internal/util"
)
type NullWriter struct{}
func (NullWriter) Write(data []byte) (n int, err error) {
	return 0, nil
}
func InitLog() {
	if util.Debug == "ON" {
		f, err := os.OpenFile("log.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		log.SetOutput(f)
		log.Println("Mecro started")
	} else {
		log.SetOutput(NullWriter{})
	}
}