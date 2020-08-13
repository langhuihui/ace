package rn

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"text/template"

	"github.com/langhuihui/ace/parse"

	"github.com/langhuihui/ace/util"

	. "github.com/langhuihui/ace/generator"
)

type Generator struct {
}

const (
	appjsTemplate = `
import React, {Component,Fragment} from "react";
import {SBaseAppComponent} from '@suning/sminip/components';
import handleAppJson from '@suning/sminip/config/appJsonConfig';
import handleAppGlobal from '@suning/sminip/config/globalApp';
import handleRouter from '@suning/sminip/config/routeConfigs';
import appJson from './app.json';
import '{{.}}./utils/snt-polyfill.js';
import SNTUtils from '{{.}}./utils/snt-utils.js';
`
	routejsTemplate = `
import { SBaseTabScreen,SView } from "@suning/sminip/components";
import lazyScreen from "./utils/sn-lazy-screen.js";
export default {
{{- if .HasTabBar -}}
	index: {
		screen: new SBaseTabScreen({
	{{- range $index,$info := .TabBar.List -}}
		{{- if eq $info.OpenType "native" -}}
				SNTabBar{{$index}}: {
                    screen: SView,
                    config:{{json $info}}
                }
		{{- else if eq $index 0 -}}
				index: {
                    screen: require('./{{$info.PagePath}}').default,
                    config: {{json $info}}
                }
		{{- else -}}
				'{{$info.PagePath}}': {
                    screen: require('./{{$info.PagePath}}').default,
                    config: {{json $info}}
                }
		{{- end -}},
	{{- end -}}
				},{
					{{- if .TabBar.Color -}}
					color:'{{.TabBar.Color}}',
					{{end}}
					{{- if .TabBar.BackgroundColor -}}
					backgroundColor:'{{.TabBar.BackgroundColor}}',
					{{end}}
					{{- if .TabBar.Position -}}
					position:'{{.TabBar.Position}}',
					{{end}}
					{{- if .TabBar.BorderStyle -}}
					borderStyle:'{{.TabBar.BorderStyle}}',
					{{end}}
					{{- if .TabBar.SelectedColor -}}
					selectedColor:'{{.TabBar.SelectedColor}}',
					{{end}}
				}).getScreen()
	}
{{- else -}}
"{{.IndexRoute}}":{screen:lazyScreen(()=>require("./{{.IndexRoute}}.js").default,()=>require("./{{.IndexRoute}}.json"))}
{{- end -}}
{{- range .RouteList -}}
,"{{.}}":{screen:lazyScreen(()=>require("./{{.}}.js").default,()=>require("./{{.}}.json"))}
{{end}}
}
`
)

var (
	appJsTemplate   *template.Template
	routeJsTemplate *template.Template
	modlueNameCache = make(util.HashSet)
	REG_SPECIAL     = regexp.MustCompile("[^\\w\\d]|_|-")
)

func init() {
	var err error
	if appJsTemplate, err = template.New("app.js").Parse(appjsTemplate); err != nil {
		log.Fatal(err)
	}
	if routeJsTemplate, err = template.New("route.js").Funcs(template.FuncMap{
		"json": func(v interface{}) string {
			r, _ := json.Marshal(v)
			return string(r)
		},
	}).Parse(routejsTemplate); err != nil {
		log.Fatal(err)
	}
}

func getModuleName(page string) string {
	pages := REG_SPECIAL.Split(page, -1)
	segment := make(util.HashSet)
	extname := ""
	for _, s := range pages {
		if len(s) == 0 {
			continue
		}
		if !segment.Has(s) {
			extname += strings.ToUpper(s[0:1]) + s[1:]
		}
		segment.Add(s)
	}
	if modlueNameCache.Has(extname) {
		extname = extname + "1"
	}
	modlueNameCache.Add(extname)
	return extname
}

func (g *Generator) GenerateRouteJs() (routeList map[string]string) {
	output, err := os.OpenFile(filepath.Join(OutputDir, "route.js"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0777)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer output.Close()
	var templateData struct {
		HasTabBar  bool
		IndexRoute string
		RouteList  []string
		TabBar     parse.TabBarInfo
	}
	routeList = map[string]string{
		"index": parse.App.Pages[0],
	}
	addRoute := func(path string) {
		if _, ok := routeList[path]; ok {
			return
		}
		routeList[path] = getModuleName(path)
		templateData.RouteList = append(templateData.RouteList, path)
	}
	if len(Entries) > 0 {
		routeList["index"] = Entries[0]
		addRoute0 := addRoute
		entryMap := make(util.HashSet)
		entryMap.AddList(Entries)
		addRoute = func(path string) {
			if entryMap.Has(path) {
				addRoute0(path)
			}
		}
	} else {
		routeList[parse.App.Pages[0]] = "index"
	}
	templateData.TabBar = parse.App.TabBar
	templateData.HasTabBar = len(parse.App.TabBar.List) > 0 && len(Entries) == 0
	templateData.IndexRoute = routeList["index"]
	for _, path := range parse.App.Pages {
		addRoute(path)
	}
	for _, path := range parse.App.SubPages {
		for _, p := range path.Pages {
			addRoute(path.Root + "/" + p)
		}
	}
	routeList[routeList["index"]] = "index"
	if err = routeJsTemplate.Execute(output,&templateData);err!=nil{
		log.Print(err)
	}
	return
}

func (g *Generator) GenerateAppJs(router map[string]string) {
	jsg := new(AppJsGenerator)
	jsg.routerBytes, _ = json.Marshal(router)
	output, err := os.OpenFile(filepath.Join(OutputDir, parse.SourceDir, "app.js"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0777)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer output.Close()
	var parser parse.JsParser
	if parse.SourceDir == "" {
		parser.Path = "app.js"
		appJsTemplate.Execute(output, "")
	} else {
		parser.Path = filepath.Join(parse.SourceDir, "app.js")
		appJsTemplate.Execute(output, ".")
	}
	jsg.output = output
	jsg.dir = ""
	parser.ParseJs(jsg)
	atomic.AddInt32(&TotalJsCount, 1)
}
func (g *Generator) CopyAssets() {
	CopyAssets()
}
