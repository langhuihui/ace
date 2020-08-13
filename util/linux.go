// +build linux darwin

package util

import (
	"log"
	"os"
	"os/exec"
	"sync"
)

func CopyFile(from string, to string) error {
	cmd := exec.Command("cp", "-f", from, to)
	cmd.Stderr = log.Writer()
	return cmd.Run()
}

func CopyFiles(files *sync.Map) error {
	fs, err := os.Create("copy.sh")
	if err != nil {
		return err
	}
	files.Range(func(k, v interface{}) bool {
		from, to := k.(string), v.(string)
		fs.WriteString("cp -f " + from + " " + to + "\n")
		return true
	})
	cmd := exec.Command("sh", "copy.sh")
	cmd.Stderr = log.Writer()
	return cmd.Run()
}
