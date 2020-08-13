package rn

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"github.com/langhuihui/ace/parse"
	"github.com/langhuihui/ace/util"
	. "github.com/langhuihui/ace/generator"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

var (
	WxsMap        = make(util.HashSet)
	WxsLock       sync.Mutex
	reg_require   = regexp.MustCompile("(?s)require\\(('|\")([^)]+\\.wxs)('|\")\\)")
)

func GenerateWxs(src string) {
	atomic.AddInt32(&TotalWxsCount, 1)
	defer PageWG.Done()
	WxsLock.Lock()
	defer WxsLock.Unlock()
	if WxsMap.Has(src) {
		return
	}
	WxsMap.Add(src)
	bytes, err := ioutil.ReadFile(filepath.Join(parse.RootDir, src))
	if err != nil {
		log.Print(err)
		return
	}
	sub := reg_require.FindAllSubmatch(bytes, -1)
	var replaceArray []string
	for _, group := range sub {
		r := string(group[2])
		rr := r
		if r[0] != '.' {
			rr = "./" + r
		}
		replaceArray = append(replaceArray, r, rr+".js")
		PageWG.Add(1)
		go GenerateWxs(filepath.Join(filepath.Dir(src), r))
	}
	if len(replaceArray) > 0 {
		if output, err := os.OpenFile(filepath.Join(OutputDir, src+".js"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666); err == nil {
			strings.NewReplacer(replaceArray...).WriteString(output, *(*string)(unsafe.Pointer(&bytes)))
			output.Close()
		} else {
			log.Print(err)
		}
	} else {
		AddAsset(src, src+".js")
	}
}
