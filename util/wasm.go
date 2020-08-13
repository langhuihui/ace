// +build js

package util

import (
	"io"
	"os"
	"sync"
)

func CopyFiles(files *sync.Map) error {
	files.Range(func(k, v interface{}) bool {
		dst, _ := os.OpenFile(v.(string), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0777)
		src, _ := os.Open(k.(string))
		io.Copy(dst, src)
		dst.Close()
		src.Close()
		return true
	})
	return nil
}
