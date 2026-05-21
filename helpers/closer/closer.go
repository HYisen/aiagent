package closer

import (
	"io"
	"log"
)

func CloseAndWarnIfFail(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Printf("warn: potential resource leak as failed to close body: %v", err)
	}
}
