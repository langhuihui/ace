package rn

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/langhuihui/ace/parse"
	. "github.com/langhuihui/ace/generator"
	"github.com/langhuihui/ace/util"
)

type Js struct {
	*parse.Page
}

var (
	AllJs        = make(util.HashSet)
	jsLock       sync.Mutex

)

func importJs(p string) {
	defer PageWG.Done()
	if !strings.HasSuffix(p, ".js") {
		p = p + ".js"
	}
	jsLock.Lock()
	defer jsLock.Unlock()
	if !AllJs.Has(p) {
		AllJs.Add(p)
		os.MkdirAll(filepath.Dir(filepath.Join(OutputDir, p)), 0777)
		output, err := os.OpenFile(filepath.Join(OutputDir, p), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0777)
		if err != nil {
			log.Print(err)
			return
		}
		defer output.Close()
		var parser parse.JsParser
		parser.Path = p
		parser.ParseJs(&CommonJsGenerator{
			output: output, dir: filepath.Dir(p),
		})
		atomic.AddInt32(&TotalJsCount, 1)
	}
}

type CommonJsGenerator struct {
	output *os.File
	dir    string
}
type ComponentJsGenerator struct {
	CommonJsGenerator
}
type PageJsGenerator struct {
	CommonJsGenerator
}
type AppJsGenerator struct {
	CommonJsGenerator
	routerBytes []byte
}

func (c *CommonJsGenerator) StartWriteConfig() {

}
func (c *CommonJsGenerator) EndWriteConfig() {

}
func (a *AppJsGenerator) StartWriteConfig() {
	a.output.WriteString("const configApp = ")
}
func (a *AppJsGenerator) EndWriteConfig() {
	route := "./route.js"
	if parse.SourceDir != "" {
		route = "../route.js"
	}
	fmt.Fprintf(a.output, `
export default class extends SBaseAppComponent {
    constructor() {
        super();
        ACEUtils.mergeSettingsToClass.call(this,configApp);
    }
 	init() {
		this.setGlobalACE();
		handleAppJson.set(appJson);
		handleAppGlobal.set(configApp);
		handleRouter.set(require('%s').default);
		this.AppNavigator = require('@suning/r/AppNavigator').default;
	}
	setGlobalACE(){
		global.ACERouter = %s;
		global.ACEUtils = ACEUtils;
	}
	renderContent() {
		const AppNavigator = this.AppNavigator;
		return (
			<AppNavigator />
		);
	}
}
`, route, a.routerBytes)
}
func (p *PageJsGenerator) StartWriteConfig() {
	p.output.WriteString(`export default class extends ACEPage {
   constructor() {
	   super();
	   ACEUtils.mergeSettingsToClass.call(this, ACEUtils.getSettings(`)
}
func (p *PageJsGenerator) EndWriteConfig() {
	p.output.WriteString(`
));
		this.setPageConfigJson(ACEJson);
		this.jsx = ACEJsx;
		this.css = ACESS
		this.css.styleMap = {}
		this.css.merge(ACEAPPCSS)
		this.css.mergeDone()
	}
}`)
}
func (c *ComponentJsGenerator) StartWriteConfig() {
	c.output.WriteString(`export default class extends ACECom {
		constructor(props, context) {
			super(props);
			const configComponent =`)
}
func (c *ComponentJsGenerator) EndWriteConfig() {
	c.output.WriteString(`
			let {behaviors,methods} = configComponent
			if(behaviors) ACEUtils.initBehaviors.call(this,configComponent);
			if(!methods) methods = configComponent.methods = {}
			ACEUtils.createBindScope.call(this, methods, 'component');
			this.initConfig(configComponent);
			this.jsx = ACEJsx;
			this.css = ACESS
			this.css.styleMap = {}
			this.css.merge(ACEAPPCSS)
			this.css.mergeDone()
			this.renderContent()
			if (this.fixedContents.length && context.addFixedComponent) {
				context.addFixedComponent(this, this.fixedContents)
			}
		}
	}`)
}
func (c *CommonJsGenerator) WriteWxAPI(s string) {
	switch s {
	case "switchTab", "redirectTo", "reLaunch", "navigateTo":
		fmt.Fprintf(c.output, "ACEUtils.%s", s)
	default:
		fmt.Fprintf(c.output, "sn.%s", s)
	}
}
func (c *CommonJsGenerator) Write(s string) {
	c.output.WriteString(s)
}
func (c *CommonJsGenerator) StartWriteBehavior() {
	c.output.WriteString("function ACEBehavior(){ return ")
}
func (c *CommonJsGenerator) EndWriteBehavior() {
	c.output.WriteString("}")
}
func (c *CommonJsGenerator) WriteImport(s string) {
	PageWG.Add(1)
	go importJs(filepath.Join(c.dir, s))
}

func (r *ReactNative) GenerateJs() {
	var parser parse.JsParser
	parser.Path = r.Path + ".js"
	var g parse.JsGenerator
	output, err := os.OpenFile(filepath.Join(OutputDir, r.Path+".js"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0777)
	if err != nil {
		log.Print(err)
		return
	}
	defer output.Close()
	utilPath := r.RelateivePath("/utils")
	fmt.Fprintf(output, `import ACECSS from "%s/ace-css";
import ACEUtils from "%s/ace-utils.js";
`, utilPath, utilPath)
	if parse.SourceDir != "" {
		fmt.Fprintf(output, `import ACEAPPCSS from "%s/app.css.js"
`, r.RelateivePath("/"+parse.SourceDir))
	} else {
		fmt.Fprintf(output, `import ACEAPPCSS from "%s/app.css.js"
`, r.RelateivePath("/"))
	}
	if r.hasWxss {
		fmt.Fprintf(output, `import ACESS from "./%s.css.js"
`, r.Name)
	} else {
		output.WriteString("const ACESS = new ACECSS([])")
	}
	if r.IsComponent {
		fmt.Fprintf(output, `
import ACECom from "%s/ace-component.js";
import {jsx as ACEJsx} from "./%s.jsx.js";
`, utilPath, r.Name)
		gg := new(ComponentJsGenerator)
		gg.output = output
		gg.dir = r.Dir
		g = gg
	} else {
		fmt.Fprintf(output, `import ACEPage from "%s/ace-page.js";
import {jsx as ACEJsx} from "./%s.jsx.js";
import ACEJson from "./%s.json";
`, utilPath, r.Name, r.Name)
		gg := new(PageJsGenerator)
		gg.output = output
		gg.dir = r.Dir
		g = gg
	}
	parser.ParseJs(g)
	atomic.AddInt32(&TotalJsCount, 1)
}
