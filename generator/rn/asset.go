package rn

import (
	"os"
	"path/filepath"

	"github.com/langhuihui/ace/parse"
	. "github.com/langhuihui/ace/generator"
)

func (g *Generator) AddAssets() {
	//filepath.Walk(filepath.Join("assets", "snt"),
	//	func(path string, info os.FileInfo, err error) error {
	//		if info.IsDir() {
	//			return nil
	//		}
	//		AddAsset(path, filepath.Join(OutputDir, "utils", info.Name()))
	//		return nil
	//	})
}
func (g *Generator) AddAsset(from string, to string) {
	from = filepath.Join(parse.RootDir, from)
	to = filepath.Join(OutputDir, to)
	AddAsset(from, to)
}

func (g *Generator) GenerateAsset(dir string) {
	defer MkdirWG.Done()
	rootDir, _ := os.Open(filepath.Join(parse.RootDir, dir))
	dirInfo, _ := rootDir.Readdir(0)
	rootDir.Close()
	var needMkDir bool
	for _, info := range dirInfo {
		name := info.Name()
		if info.IsDir() {
			MkdirWG.Add(1)
			go g.GenerateAsset(filepath.Join(dir, name))
		} else if name != "app.js" {
			p := filepath.Join(dir, name)
			ext := filepath.Ext(name)
			switch ext {
			case ".wxs":
				needMkDir = true
				continue
			case ".js":
				needMkDir = true
				//p := p[:len(p)-len(ext)]
				//_, ok1 := parse.App.PagesMap[p]
				//_, ok2 := parse.AllComponents[p]
				//if !ok1 && !ok2 {
				//
				//}
				continue
			case ".json":
				p := p[:len(p)-len(ext)]
				_, ok2 := parse.AllComponents[p]
				if parse.App.HasPage(p) || ok2 {

				} else {
					continue
				}
			case ".jpg", ".png", ".gif", ".bmp", ".jpeg", ".svg":
				needMkDir = true
			default:
				continue
			}
			g.AddAsset(p, p)
		}
	}
	if needMkDir {
		os.MkdirAll(filepath.Join(OutputDir, dir), 0775)
	}
}
