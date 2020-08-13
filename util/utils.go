package util

import (
	"path/filepath"
	"regexp"
	"strings"
)

type HashSet map[string]struct{}

func (set HashSet) Add(s string) {
	set[s] = struct{}{}
}
func (set HashSet) Has(s string) bool {
	_, ok := set[s]
	return ok
}
func (set HashSet) ToList() (list []string) {
	for s := range set {
		list = append(list, s)
	}
	return
}
func (set HashSet) AddList(list []string) {
	for _, s := range list {
		set.Add(s)
	}
}
func (set HashSet) Filter(f func(string) bool) (list []string) {
	for s := range set {
		if f(s) {
			list = append(list, s)
		}
	}
	return
}
func (set HashSet) Assign(set2 HashSet) {
	for s := range set2 {
		set.Add(s)
	}
}

// RelateivePath 获取相对Page的路径的相对路径
func RelateivePath(base string, p string) string {
	if p[0] == '/' {
		p, _ = filepath.Rel("/"+base, p)
	} else {
		//p ,_= filepath.Rel(base, p)
	}
	p = strings.ReplaceAll(p, "\\", "/")
	if p[0] != '.' {
		return "./" + p
	}
	return p
}
func CamelCase(ss []string) (k string) {
	if len(ss) == 1 {
		return ss[0]
	}
	k = ss[0]
	for _, s := range ss[1:] {
		k += strings.ToUpper(s[0:1]) + s[1:]
	}
	return
}

var RegNumberic = regexp.MustCompile("^[0-9.]+$")
var RegNotNumberic = regexp.MustCompile("^0[0-9.]+$")
