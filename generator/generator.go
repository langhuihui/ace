package generator

import (
	"github.com/langhuihui/ace/parse"
	"github.com/langhuihui/ace/util"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Entry []string

func (*Entry) String() string {
	return ""
}
func (e *Entry) Set(str string) error {
	*e = strings.Split(str, ",")
	return nil
}

var (
	DepthIndent        []string
	PageWG             sync.WaitGroup
	MkdirWG            sync.WaitGroup
	assets             sync.Map
	GenerateType       string //生成的目标类型
	OutputDir          string
	Entries            Entry //用于支持部分页面转码
	TagsMap            = make(map[string]string)
	AllComponents      = make(util.HashSet)
	AllImports         = make(util.HashSet)
	ImportLock         sync.Mutex
	TotalCount         int32 //总生成页面数
	TotalJsCount       int32 //总的js生成数
	TotalWxsCount      int32 //总的wxs转换数
	TotalCSSCount      int32 //总的css生成数
	HasAppWxss         bool
	maxDepth           = 50
)

func init() {
	DepthIndent = make([]string, maxDepth)
	for i := 1; i < maxDepth; i++ {
		DepthIndent[i] = strings.Repeat("  ", i)
	}
	if bytes, err := ioutil.ReadFile("tags.yaml"); err == nil {
		if err = yaml.Unmarshal(bytes, TagsMap);err!=nil{
			log.Print(err)
		}
	}
}

type IGenerator interface {
	GenerateRouteJs() map[string]string
	GenerateAppJs(map[string]string)
	GenerateCss(string) error
	GeneratePage(*parse.Page)
	AddAsset(string, string)
	AddAssets()
	CopyAssets()
	GenerateAsset(string)
}

func GenerateApp(generator IGenerator) {
	generator.GenerateAppJs(generator.GenerateRouteJs())
	if parse.SourceDir != "" {
		if generator.GenerateCss(filepath.Join(parse.SourceDir, "app")) == nil {
			HasAppWxss = true
		}
		generator.AddAsset(filepath.Join(parse.SourceDir, "app.json"), filepath.Join(parse.SourceDir, "app.json"))
	} else {
		if generator.GenerateCss("app") == nil {
			HasAppWxss = true
		}
		generator.AddAsset("app.json", "app.json")
	}
}

func CopyAssets() {
	util.CopyFiles(&assets)
}
func AddAsset(from string, to string) {
	if f, e := os.Stat(to); e == nil {
		toT := f.ModTime()
		f, e = os.Stat(from)
		if f.ModTime() == toT {
			return
		}
	}
	//os.MkdirAll(filepath.Dir(to), 0777)
	if strings.ContainsAny(from, " ") {
		from = `"` + from + `"`
	}
	if strings.ContainsAny(to, " ") {
		to = `"` + to + `"`
	}
	assets.Store(from, to)
}
