package pkg

import (
	"fmt"
	"io"
	"os"
)

func SaveToFile(file string, data string) error {
	if file != "" {
		f, err := os.OpenFile(file, os.O_WRONLY|os.O_APPEND, os.FileMode(0644))
		if err != nil {
			return err
		}
		n, err := f.Write([]byte(data))
		if err == nil && n < len(data) {
			err = io.ErrShortWrite
		}
		if err1 := f.Close(); err == nil {
			err = err1
		}
		return err
	}
	return fmt.Errorf("invalid file name")
}
