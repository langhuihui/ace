// +build windows

package util

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
)

func CopyFile(from string, to string) error {
	cmd := exec.Command("cmd", "/C", "copy", "/Y", "/B", from, "/B", to)
	cmd.Stderr = log.Writer()
	return cmd.Run()
}

func CopyFiles(files *sync.Map) error {
	fs, err := os.Create("copy.bat")
	if err != nil {
		return err
	}
	files.Range(func(k, v interface{}) bool {
		fmt.Fprintf(fs, "copy /Y %s %s\r\n", k.(string), v.(string))
		return true
	})
	fs.Close()
	cmd := exec.Command("cmd", "/C", "copy.bat")
	cmd.Stderr = log.Writer()
	return cmd.Run()
}
