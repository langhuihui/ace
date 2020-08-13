package parse

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/langhuihui/ace/util"
)

var (
	AllComponents = make(map[string]*Page)
	Includes      = make(map[string]Generator)
	AllImports    = make(util.HashSet)
	includeLock   sync.Mutex
	importLock    sync.Mutex
)

type Generator interface {
	Create(*Page) Generator
	GetTagName(string) string
	//WriteText()
	//WriteChild(Generator)
	//WriteTemplate()
	//WriteElse()
	//WriteEndIf()
	//WriteEndElse()
	//WriteFor()
	//WriteIf()
	//WriteElseIf(string)
	//WriteTagStart()
	//WriteTagStartClose()
	//WriteAttr(string, *Attribute)
	//WriteExitTag() bool
	//WriteEndTag()
	//WriteEndFor()
	GenerateImport(string)
	Wait()
	Done()
}

//type PageParseContext struct {
//	CurrentEl  *WxmlEl //当前正在转换的元素
//}
type Page struct {
	WxmlEl          //根元素
	Path            string
	Dir             string
	Name            string
	UsingComponents map[string]string
	IsComponent     bool `json:"component"`
	//PageParseContext
	Templates       map[string]*WxmlEl //内联模板定义
	Imports         []string           //导入的页面
	Wxs             map[string]string
	isInclude       bool
	HasTemplateCall bool      //是否有模板调用
	Generated       sync.Once //只需生成一次
}

//去除两边引号
func rb(str string) string {
	return str[1 : len(str)-1]
}
func (page *Page) Unmarshal(path string) error {
	page.Dir, page.Name = filepath.Split(path)
	page.Path = filepath.Join(page.Dir, page.Name)
	bytes, err := ioutil.ReadFile(filepath.Join(RootDir, path+".json"))
	if os.IsNotExist(err) {
		if bytes, err = ioutil.ReadFile(filepath.Join(RootDir, path, "index.json")); err != nil {
			return err
		} else {
			page.Dir = page.Path
			page.Path = filepath.Join(page.Path, "index")
			page.Name = "index"
		}
	}
	if err = json.Unmarshal(bytes, page); err != nil {
		return err
	}
	if page.IsComponent {
		AllComponents[page.Path] = page
	}
	for name, relativePath := range page.UsingComponents {
		if strings.HasPrefix(relativePath, "plugin:") {
			log.Printf("%s not support plugin", filepath.Join(RootDir, page.Path+".json"))
			delete(page.UsingComponents, name)
			continue
		}
		path = page.ParsePath(relativePath)
		if _, ok := AllComponents[path]; !ok {
			var component Page
			if err = component.Unmarshal(path); os.IsNotExist(err) {
				log.Printf("%s usingComponent %s json not exit", filepath.Join(RootDir, page.Path+".json"), name)
			}
			if !component.IsComponent {
				log.Printf("%s should set component true:  %s", filepath.Join(RootDir, page.Path+".json"), path)
				component.IsComponent = true
			}
			if component.Path != path {
				relativePath = relativePath + "/index"
				page.UsingComponents[name] = relativePath
				AllComponents[relativePath] = &component
			}
			AllComponents[path] = &component
		}
	}
	return nil
}

// ParsePath 获取相对Root的路径
func (page *Page) ParsePath(path string) string {
	if path[0] == '/' {
		return filepath.Join(path[1:])
	}
	return filepath.Join(page.Dir, path)
}

// RelateivePath 获取相对Page的路径的相对路径
func (page *Page) RelateivePath(p string) string {
	return util.RelateivePath(page.Dir, p)
}
func (page *Page) printErr(el *WxmlEl, err string) {
	log.Printf("%s.wxml:%d:%d %s", filepath.Join(RootDir, page.Path), el.Line, el.Column, err)
}

func (page *Page) ParseWxml(g Generator) error {
	input, err := os.Open(filepath.Join(RootDir, page.Path+".wxml"))
	if err != nil {
		log.Print(err)
		return err
	}
	page.Templates = make(map[string]*WxmlEl)
	page.Wxs = make(map[string]string)
	//root := &WxmlEl{
	//	TagName:  "page",
	//	Attrs:    make(map[string]Attribute),
	//	Children: make([]*WxmlEl, 0),
	//	HashCode: "1",
	//}
	page.Root = &page.WxmlEl
	page.Identifiers = make(util.HashSet)
	page.TagName = "page"
	page.Attrs = make(map[string]*Attribute)
	page.Children = make([]*WxmlEl, 0)
	page.HashCode = "1"
	//writeIf := func(el *WxmlEl) {
	//	switch el.Condition {
	//	case COND_IF:
	//		g.WriteIf()
	//		fallthrough
	//	case COND_ELIF:
	//		g.WriteElseIf(Bracket(el.Value))
	//	}
	//}
	enter := func(el *WxmlEl) {
		//page.CurrentEl = el
		//idMap := page.Identifiers
		//if el.Template != nil {
		//	idMap = el.Template.Identifiers
		//}
		//for identifier := range el.Identifiers {
		//	idMap.Add(identifier)
		//}
		_, el.CustomComponent = page.UsingComponents[el.TagName]
		el.TargetTagName = g.GetTagName(el.TagName)
		//处理前一个同级节点收尾工作
		//if el.Previous != nil && el.Previous.For == "" {
		//	page.CurrentEl = el.Previous
		//	//根据前一个同级节点判断条件语句
		//	switch el.Previous.Condition {
		//	case COND_IF, COND_ELIF:
		//		switch el.Condition {
		//		case COND_ELIF, COND_ELSE:
		//			g.WriteElse()
		//		default:
		//			g.WriteEndIf()
		//		}
		//	case COND_ELSE:
		//		g.WriteEndElse()
		//	}
		//	page.CurrentEl = el
		//}
		switch el.TagName {
		case "": //处理文本节点
			//g.WriteText()
		case "include":
			targetPath := page.ParsePath(el.Attrs["src"].Value)
			includeLock.Lock()
			iG, ok := Includes[targetPath]
			var iPage *Page
			if !ok {
				iPage = getPage(targetPath)
				iPage.isInclude = true
				iG = g.Create(iPage)
				Includes[targetPath] = iG
			}
			includeLock.Unlock()
			if !ok {
				defer iG.Done()
				if err = iPage.ParseWxml(iG); os.IsNotExist(err) {
					page.printErr(el, "include file not exist")
					return
				}
			} else {
				iG.Wait()
			}
			if el.Template == nil {
				el.Root.Identifiers.Assign(iPage.Identifiers)
			} else {
				el.Template.Identifiers.Assign(iPage.Identifiers)
			}
			//g.WriteChild(iG)
			el.Parent.Children = append(el.Parent.Children[:len(el.Parent.Children)-1], iPage.Children...)
		case "import":
			page.Imports = append(page.Imports, el.Attrs["src"].Value)
			targetPath := page.ParsePath(el.Attrs["src"].Value)
			importLock.Lock()
			if !AllImports.Has(targetPath) {
				AllImports.Add(targetPath)
				importLock.Unlock()
				iPage := getPage(targetPath)
				iG := g.Create(iPage)
				iG.GenerateImport(targetPath)
			} else {
				importLock.Unlock()
			}
			el.Parent.Children = el.Parent.Children[:len(el.Parent.Children)-1]
		case "template":
			if name, ok := el.Attrs["name"]; ok {
				page.Templates[name.Value] = el
				return
			} else {
				el.TagName = "template:Call"
				page.HasTemplateCall = true
				//writeIf(el)
			}
		case "wxs":
		case "slot":
			//writeIf(el)
		default:
			////处理循环
			//if el.For != "" {
			//	g.WriteFor()
			//}
			////处理条件
			//writeIf(el)
			//g.WriteTagStart()
			////写入属性
			//for k, v := range el.Attrs {
			//	g.WriteAttr(k, &v)
			//}
			//g.WriteTagStartClose()
		}
	}
	//endParse := func(el *WxmlEl) {
	//	if last := el.LastChild(); last != nil && last.For == "" {
	//		page.CurrentEl = last
	//		switch last.Condition {
	//		case COND_ELSE:
	//			g.WriteEndElse()
	//		case COND_IF, COND_ELIF:
	//			g.WriteEndIf()
	//		}
	//		page.CurrentEl = el
	//	}
	//}
	exit := func(el *WxmlEl) {
		//page.CurrentEl = el
		switch el.TagName {
		case "include":
			return
		case "wxs":
			if page.isInclude {
				return
			}
			fallthrough
		default:
			//if g.WriteExitTag() {
			//	return
			//}
		}
		//endParse(el)
		//switch el.TagName {
		//case "template", "slot", "template:Call":
		//default:
		//	if el.Children != nil || el.Text != "" {
		//		g.WriteEndTag()
		//	}
		//}
		//if el.For != "" {
		//	if el.Condition == COND_IF {
		//		g.WriteEndIf()
		//	}
		//	g.WriteEndFor()
		//}
		if el.TagName == "template" {
			//page.InTemplate = nil
			//g.WriteTemplate()
		}
	}
	if page.IsComponent {
		page.Parse(input, enter, exit)
		//endParse(root)
		//page.CurrentEl = root
	} else {
		enter(page.Root)
		page.Parse(input, enter, exit)
		exit(page.Root)
	}
	input.Close()
	//endParse(root)
	return nil
}
func getPage(targetPath string) *Page {
	p := strings.TrimSuffix(targetPath, ".wxml")
	page, ok := App.PagesMap[p]
	if !ok {
		page = new(Page)
		page.Unmarshal(p)
	}
	return page
}
