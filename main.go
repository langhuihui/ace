package main

import (
	"flag"
	"github.com/langhuihui/ace/generator/rn"
	"log"
	"os"
	"path/filepath"
	"time"

	. "github.com/langhuihui/ace/generator"
	"github.com/langhuihui/ace/parse"
)
func CreateGenerator() IGenerator {
	switch GenerateType {
	case "rn":
		return &rn.Generator{}
	}
	return nil
}
func main() {
	//入口参数，可空，用来转换部分路由，方便用于测试
	flag.Var(&Entries, "entry", "wechat minip entries")
	//根路径，所有源码和资源的根目录，用于多个小程序公共资源访问
	flag.StringVar(&parse.RootDir, "root", "", "wechat minip root dir")
	//微信小程序源码目录，必须包含app.json等
	flag.StringVar(&parse.SourceDir, "source", "", "wechat minip source dir")
	//输出目录，用于转换后的代码和资源的输出
	flag.StringVar(&OutputDir, "output", "", "target output dir")
	//是否清空输出目录
	clear := flag.Bool("clear", false, "clear output dir")
	//转换目标类型
	flag.StringVar(&GenerateType, "type", "rn", "target output type")
	flag.Parse()
	if *clear {
		os.RemoveAll(OutputDir)
	}
	//创建输出目录
	os.MkdirAll(OutputDir, 0755)
	//转换起始时间，用于计算总共花费时间
	start := time.Now()
	//对所有源码中的json配置进行读取，建立源码引用关系和基本数据结构
	parse.App.Unmarshal()
	g := CreateGenerator()
	//转换App相关资源
	GenerateApp(g)
	//递归创建资源目录，并行创建
	MkdirWG.Add(1)
	g.GenerateAsset("")
	MkdirWG.Wait()
	log.Printf("mkdir finished %dms since start", time.Since(start)/time.Millisecond)
	if len(Entries) > 0 {
		//执行部分转换
		PageWG.Add(len(Entries))
		for _, page := range Entries {
			go g.GeneratePage(parse.App.PagesMap[filepath.Join(page)])
		}
	} else {
		//执行全量转换
		PageWG.Add(len(parse.App.PagesMap))
		for _, page := range parse.App.PagesMap {
			go g.GeneratePage(page)
		}
	}
	PageWG.Wait()
	log.Printf("total page:%d,total js:%d,total wxs:%d,total css.js:%d, %dms since start", TotalCount, TotalJsCount, TotalWxsCount, TotalCSSCount, time.Since(start)/time.Millisecond)
	//其他资源生成待拷贝列表
	g.AddAssets()
	//将待拷贝列表写入批处理文件并执行
	g.CopyAssets()
	log.Printf("finished %dms since start", time.Since(start)/time.Millisecond)
}
