package common

import (
	"fmt"
	"log"
	"os"
)

func CreateLoggers(name string) (info *log.Logger, error *log.Logger) {
	return log.New(os.Stdout,
			fmt.Sprintf("[INFO] [%s] ", name),
			log.Ldate|log.Ltime|log.Lmsgprefix),
		log.New(os.Stderr,
			fmt.Sprintf("[ERROR] [%s] ", name),
			log.Ldate|log.Ltime|log.Lmsgprefix)
}