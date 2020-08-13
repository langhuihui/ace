package rn

import (
	"fmt"
	. "github.com/langhuihui/ace/generator"
	"github.com/langhuihui/ace/parse"
	"github.com/langhuihui/ace/util"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
)

var (
	reg_binary = regexp.MustCompile("[^10]")
	reg_image  = regexp.MustCompile("^(http|file|data):")
)

const JSXTemplate = `
{{- define "jsxTag" -}}
{{- $el:=. -}}
{{- if .TagName}}
{{indent .}}<{{.TargetTagName}}{{range $k,$v := .Attrs}}{{attr $el $k $v}}{{end}}
{{- if gt (len .Children) 0}}>{{range .Children}}{{render .}}{{end}}
{{indent .}}</{{.TargetTagName}}>{{else}}/>{{end -}}
{{- else if eq .Parent.TagName "text"}}{{.Text}}{{else -}}
<{{getTagName "text"}} style={GS({{getStyleHashCode .}})}>{{.Text}}</{{getTagName "text"}}>
{{- end -}}
{{- end -}}

{{- define "jsxTagIf" -}}
{{- if eq .Condition 0 -}}
{{template "jsxTag" . -}}
{{- else if or (eq .Condition 1) (eq .Condition 2)}}
{{indent .}}{
{{- .Value}}?{{template "jsxTag" .}}
{{indent .}}:{{if .Next}}{{if lt .Next.Condition 2}}null}{{end}}{{else}}null}{{end -}}
{{- else}}{{template "jsxTag" .}}
{{indent .}}}{{end -}}
{{- end -}}

{{- define "jsxTagFor" -}}
{{- if .For}}
{{indent .}}{
{{- if numberic .For -}}
(new Array({{.For}})).map(({{.Item}},{{.Index}})=>{
{{- else -}}
ACEUtils.forEach({{.For}},({{.Item}},{{.Index}})=>{
{{- end}}
{{- if and .Parent .Parent.InFor}}
{{indent .}}const HC{{.ForDepth}} = {{if .Template}}HC+{{end}}HC{{forDepth1 .}}+'{{trimPrefix .HashCode .Parent.InFor.HashCode}}|'+GHC({{.Index}})
{{- else}}
{{indent .}}const HC1 = {{if .Template}}HC+{{end}}'{{.HashCode}}|'+GHC({{.Index}})
{{- end}}{{if eq .TagName "block"}}+'|'{{end}}
{{indent .}}return (
{{- template "jsxTagIf" .}}
{{indent .}})})}
{{- else}}{{template "jsxTagIf" .}}{{end}}
{{- end -}}
`

type ReactNative struct {
	*parse.Page
	*Generator
	strings.Builder
	line, column, offset, currentCount int
	components                         util.HashSet
	usingComponents                    map[string]string //真正使用到的自定义组件
	parentHash                         string
	sync.WaitGroup
	hasWxss     bool
	jsxTemplate *template.Template
}

func (r *ReactNative) GetComponentName(tagName string) string {
	return util.CamelCase(strings.Split(tagName, "-"))
}

func (r *ReactNative) WriteComponent(tagName string, component string) {
	path := r.ParsePath(component)
	cp, ok := parse.AllComponents[path]
	if !ok {
		log.Fatal(path, " impossible!")
		//cp = new(parse.Page)
		//cp.Unmarshal(filepath.Join(r.Dir,component))
		//parse.AllComponents[component] = cp
	}
	r.usingComponents[tagName] = component
	cp.Generated.Do(func() {
		PageWG.Add(1)
		go r.GeneratePage(cp)
	})
}

//func (r *ReactNative) writef(format string, a ...interface{}) {
//	r.write(fmt.Sprintf(format, a...))
//}
//func (r *ReactNative) write(s string) {
//	if r.InTemplate != nil {
//		r.InTemplate.Text += s
//	} else {
//		n, _ := r.WriteString(s)
//		r.offset += n
//	}
//}
//func (r *ReactNative) writeln(s string) {
//	if r.write(s + "\n"); r.InTemplate == nil {
//		r.line++
//		r.column = r.offset
//	}
//}
//func (r *ReactNative) writeTab() {
//	r.write("\n")
//	depth := r.CurrentEl.Depth
//	if depth > 0 {
//		r.write(DepthIndent[depth])
//	}
//}

func (r *ReactNative) GetTagName(tagName string) string {
	if component, ok := r.UsingComponents[tagName]; ok {
		r.WriteComponent(tagName, component)
		return r.GetComponentName(tagName)
	}
	if t, ok := TagsMap[tagName]; ok {
		if t != "Fragment" {
			r.components.Add(t)
		}
		return t
	}
	if tagName == "" {
		r.components.Add("Text")
	}
	return tagName
}

func (r *ReactNative) WriteAttr(el *parse.WxmlEl, k string, v *parse.Attribute) string {
	rnAttrVal := "{" + v.Value + "}"
	if v.IsPureStr {
		rnAttrVal = v.Raw
	}
	switch {
	case k == "id" && el.CustomComponent:
		//r.write(" ref=" + rnAttrVal)
	case k == "wx:key":
		k = "key"
		if v.Value == "*this" {
			rnAttrVal = "{" + el.InFor.Item + "}"
		} else if v.IsPureStr && v.Value != el.InFor.Index {
			rnAttrVal = "{" + el.InFor.Item + "[" + v.Raw + "]}"
		} else {
			rnAttrVal = "{" + v.Value + "}"
		}
	case k == "class":
		return ""
	case k == "style":
		return ""
	case k == "src" && el.TagName == "image":
		if !v.IsPureStr || !reg_image.MatchString(v.Value) {
			rnAttrVal = fmt.Sprintf("{ACEUtils.getImageUrl(%s,PAGEPATH)}", v.JsxValue)
		}
	case k == "url" && el.TagName == "navigator":
		if v.Value == "" {
			break
		}
		if v.Value[0] == '/' && v.IsPureStr {
			rnAttrVal = v.Raw[:1] + v.Value[1:] + v.Raw[:1]
		} else {
			rnAttrVal = fmt.Sprintf("{ACEUtils.getNavigatorUrl(%s,'/%s')}", v.JsxValue, strings.ReplaceAll(r.Path, "\\", "/"))
		}
	//case strings.HasPrefix(k, "wx-"), strings.HasPrefix(k, "ace-"):
	case strings.HasPrefix(k, "data-"):
		ss := strings.Split(k, "-")
		if k = "data-" + strings.ToLower(ss[1]); len(ss) > 2 {
			k += util.CamelCase(ss[2:])
		}
		//事件绑定
	case strings.HasPrefix(k, "bind"), strings.HasPrefix(k, "catch"):
		switch v.Value {
		case "":
			return ""
		default:
			rnAttrVal = "{this[" + v.JsxValue + "]}"
		}
		if ss := strings.Split(k, ":"); len(ss) > 1 {
			k = ss[0] + util.CamelCase(strings.Split(ss[1], "-"))
		}
		k = "on" + k
	case strings.Contains(k, ":"):
		kk := strings.Split(k, ":")
		k = kk[0] + util.CamelCase(kk[1:])
	default:
		k = util.CamelCase(strings.Split(k, "-"))
	}
	if v.Value == "false" {
		rnAttrVal = "{false}"
	} else if v.Value == "true" || v.Raw == "" {
		return " " + k
	}
	return " " + k + "=" + rnAttrVal
}

func (r *ReactNative) Create(page *parse.Page) parse.Generator {
	result := &ReactNative{
		Page:            page,
		components:      make(util.HashSet),
		usingComponents: make(map[string]string),
		jsxTemplate:     template.New("jsx"),
	}
	result.Add(1)
	return result
}

//func (r *ReactNative) WriteTemplate() {
//	el := r.CurrentEl
//	if len(el.Children) > 1 || len(el.Children) == 1 && (el.Children[0].Condition > 0 || el.Children[0].For != "") {
//		el.Text = "<Fragment>\n" + el.Text + "</Fragment>"
//	}
//}

func (r *ReactNative) getStyleHashCode(el *parse.WxmlEl) (result string) {
	if el.Template != nil || el.ForDepth > 0 {
		if el.ForDepth > 0 {
			result = fmt.Sprintf("HC%d+'%s'", el.ForDepth, strings.TrimPrefix(el.HashCode, el.InFor.HashCode))
		}
		if el.Template != nil {
			result = fmt.Sprintf("HC+'%s'", el.HashCode)
		}
	} else if reg_binary.MatchString(el.HashCode) {
		result = `'` + el.HashCode + `'`
	} else {
		i, _ := strconv.ParseInt(el.HashCode, 2, 32)
		result = strconv.Itoa(int(i))
	}
	return
}

//func (r *ReactNative) WriteTagStartClose() {
//	el := r.CurrentEl
//	if el.TagName != "block" {
//		gsParams := []string{
//			r.getStyleHashCode(el),
//			`'` + el.TagName + `'`,
//			"''", "''", "''",
//		}
//		emptyCount := 3
//		for i, n := range []string{"class", "id", "style"} {
//			if c, ok := el.Attrs[n]; ok {
//				gsParams[i+2] = c.JsxValue
//				emptyCount = 2 - i
//			}
//		}
//		r.writef(" style={GS(%s)}", strings.Join(gsParams[:5-emptyCount], ","))
//	}
//	if el.CustomComponent {
//		//r.write(" onChildRef={this.onChildRef}")
//		var sntComponentKeys []string
//		for inFor := el.InFor; inFor != nil; inFor = inFor.Parent.InFor {
//			sntComponentKeys = append(sntComponentKeys, inFor.Index)
//			if inFor.Parent == nil {
//				break
//			}
//		}
//		if sntComponentKeys != nil {
//			r.write(" aceComponentKey={" + strings.Join(sntComponentKeys, "+'|'+") + "}")
//		}
//	}
//	if el.Children == nil {
//		r.write(" />")
//	} else {
//		r.write(">")
//	}
//}
//
//func (r *ReactNative) WriteExitTag() bool {
//	el := r.CurrentEl
//	switch el.TagName {
//	case "wxs":
//		module := el.Attrs["module"].Value
//		if src, ok := el.Attrs["src"]; ok {
//			src := src.Value
//			filePath := filepath.Join(r.Dir, src)
//			if _, e := os.Stat(filepath.Join(parse.RootDir, filePath)); os.IsNotExist(e) {
//				log.Printf(filepath.Join(parse.RootDir, r.Path+".wxml")+":%d wxs不存在", el.Line)
//			} else {
//				r.Wxs[module] = src
//				PageWG.Add(1)
//				go GenerateWxs(filePath)
//			}
//		} else {
//			r.Wxs[module] = "./wxs/" + module
//			dir := filepath.Join(OutputDir, r.Dir, "wxs")
//			os.MkdirAll(dir, 0775)
//			if err := ioutil.WriteFile(filepath.Join(dir, module+".js"), []byte(el.Text), 0666); err != nil {
//				log.Print(err)
//			}
//		}
//		return true
//	case "template:Call":
//		if el.Condition == 0 {
//			r.writeTab()
//			r.write("{")
//			defer r.write("}")
//		}
//		is := el.Attrs["is"]
//		delete(el.Attrs, "is")
//		el.TargetTagName = "Fragment"
//		data, ok := el.Attrs["data"]
//		if ok {
//			delete(el.Attrs, "data")
//			r.writef("renderTemplate.call(this,%s,%s,GS,'%s|')", is.JsxValue, data.Raw[2:len(data.Raw)-2], el.HashCode)
//		} else {
//			r.writef("renderTemplate.call(this,%s,{},GS,'%s|')", is.JsxValue, el.HashCode)
//		}
//	case "slot":
//		if el.Condition == 0 {
//			r.write("{")
//			defer r.write("}")
//			el.Parent.RemoveChild(el)
//		}
//		if name, ok := el.Attrs["name"]; ok {
//			r.writef("this.renderSSlot({name:%s})", name.JsxValue)
//		} else {
//			r.writef("this.renderSSlot()")
//		}
//		//el.Parent.RemoveChild(el)
//	}
//	return false
//}
//
//func (r *ReactNative) WriteEndTag() {
//	el := r.CurrentEl
//	if len(el.Children) > 0 {
//		r.writeTab()
//	}
//	r.write("</" + el.TargetTagName + ">")
//}
//
//func (r *ReactNative) WriteWxs() {
//	r.writeTab()
//	r.write("</" + r.CurrentEl.TargetTagName + ">")
//}
func (r *ReactNative) WriteJsxHead(output *os.File) {
	fmt.Fprintln(output, `import React, {Fragment} from "react";`)
	fmt.Fprintf(output, "import {%s} from \"@suning/r/components\";\n", strings.Join(r.components.ToList(), ","))
	fmt.Fprintf(output, "import ACEUtils from '%s'\n", r.RelateivePath("/utils/ace-utils.js"))
	for name, v := range r.Wxs {
		fmt.Fprintf(output, "import %s from \"%s.js\"\n", name, v)
	}
	for name, path := range r.usingComponents {
		fmt.Fprintf(output, "import %s from \"%s.js\"\n", r.GetComponentName(name), r.RelateivePath(path))
	}
	fmt.Fprintf(output, "const PAGEPATH = 'rnCode/%s'\n", strings.ReplaceAll(r.Dir, "\\", "/"))
	var templates []string
	if len(r.Templates) > 0 {
		templates = append(templates, "Templates[name]")
		output.WriteString("export const Templates = {\n")
		for name, el := range r.Templates {
			fmt.Fprintf(output, `"%s":function({%s},GS,HC){ return (
	%s
	)},`, name, strings.Join(el.Identifiers.ToList(), ","), el.Text)
		}
		output.WriteString("}\n")
	}
	if len(r.Imports) > 0 {
		templates = append(templates, "ImportTemplates[name]")
		output.WriteString("const ImportTemplates = {\n")
		for _, src := range r.Imports {
			fmt.Fprintf(output, "...require('%sjsx.js').Templates,", strings.TrimSuffix(src, "wxml"))
		}
		output.WriteString("}\n")
	}
	if r.HasTemplateCall {
		fmt.Fprintf(output, `function renderTemplate(name, args,gs, hc) {
	return (%s).call(this,args, gs, hc)
}`, strings.Join(templates, "||"))
	}
}

//生成page和component页面
func (g *Generator) GeneratePage(page *parse.Page) {
	r := &ReactNative{Page: page, Generator: g}
	defer PageWG.Done()
	r.usingComponents = make(map[string]string)
	r.components = make(util.HashSet)
	r.jsxTemplate = template.New("jsx")
	if err := r.ParseWxml(r); os.IsNotExist(err) {
		return
	}
	r.GenerateCss()
	r.GenerateJs()
	output, _ := os.OpenFile(filepath.Join(OutputDir, r.Path+".jsx.js"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0777)
	defer output.Close()
	_, err := r.jsxTemplate.Funcs(template.FuncMap{
		"trimPrefix": strings.TrimPrefix,
		"render": func(el *parse.WxmlEl) string {
			var buffer strings.Builder
			r.jsxTemplate.ExecuteTemplate(&buffer, "jsxTagFor", el)
			return buffer.String()
		},
		"indent": func(el *parse.WxmlEl) string {
			depth := el.Depth
			if depth > 0 {
				return DepthIndent[depth]
			} else {
				return ""
			}
		},
		"getTagName": func(tagName string) string {
			return TagsMap[tagName]
		},
		"attr":             r.WriteAttr,
		"getStyleHashCode": r.getStyleHashCode,
		"numberic":         util.RegNumberic.MatchString,
		"forDepth1": func(el *parse.WxmlEl) int {
			return el.ForDepth - 1
		},
	}).Parse(JSXTemplate)
	if err != nil {
		log.Print(err)
	}
	if err = r.jsxTemplate.ExecuteTemplate(output, "jsxTagFor", r.Root); err != nil {
		log.Print(err)
	}
	//r.WriteJsxHead(output)
	//bodyContent := r.String()
	//if r.IsComponent {
	//	r.Identifiers.Add("aceComponentKey=''")
	//	count := 0
	//	for _, child := range r.Children {
	//		if child.Condition < 2 {
	//			if count++; count > 1 {
	//				bodyContent = "<Fragment>\n" + bodyContent + "\n</Fragment>"
	//				break
	//			}
	//		}
	//	}
	//	if count == 1 {
	//		bodyContent = strings.Trim(strings.TrimSpace(bodyContent), " \n{}")
	//	}
	//}
	jsxArgs := []string{"GS", "GHC"}
	identifiers := r.Identifiers.Filter(func(ident string) bool {
		_, ok := r.Wxs[ident]
		return !ok
	})
	if len(identifiers) > 0 {
		jsxArgs = append(jsxArgs, fmt.Sprintf("{%s}", strings.Join(identifiers, ",")))
	}
	//if bodyContent == "" {
	//	bodyContent = "null"
	//}
	//	fmt.Fprintf(output, `
	//export function jsx(%s) {
	//	return (
	//%s)
	//}`, strings.Join(jsxArgs, ","), bodyContent)
	atomic.AddInt32(&TotalCount, 1)
}

//生成模板引入的wxml
func (r *ReactNative) GenerateImport(src string) {
	p := strings.TrimSuffix(src, ".wxml")
	if parse.App.HasPage(p) {
		return
	}
	ImportLock.Lock()
	if !AllImports.Has(p) {
		r.Page = new(parse.Page)
		r.Page.Unmarshal(p)
		AllImports.Add(p)
		ImportLock.Unlock()
	} else {
		//避免重复生成
		ImportLock.Unlock()
		return
	}
	r.usingComponents = make(map[string]string)
	r.components = make(util.HashSet)
	if err := r.ParseWxml(r); os.IsNotExist(err) {
		return
	}
	os.MkdirAll(filepath.Join(OutputDir, r.Dir), 0777)
	output, _ := os.OpenFile(filepath.Join(OutputDir, r.Path+".jsx.js"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0777)
	defer output.Close()
	r.WriteJsxHead(output)
}
