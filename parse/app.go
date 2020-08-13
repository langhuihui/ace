package parse

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
)

var (
	RootDir   string
	SourceDir string
	App       AppInfo
)

type TabBarInfo struct {
	Position        string
	Color           string
	SelectedColor   string
	BorderStyle     string
	BackgroundColor string
	List            []struct {
		PagePath         string `json:"pagePath"`
		Text             string `json:"text"`
		IconPath         string `json:"iconPath"`
		SelectedIconPath string `json:"selectedIconPath"`
		OpenType         string `json:"openType"`
	}
}
type AppInfo struct {
	Pages    []string
	SubPages []struct {
		Root  string
		Pages []string
	} `json:"subPackages"`
	TabBar   TabBarInfo
	SnTabBar TabBarInfo
	PagesMap map[string]*Page
}

func (app *AppInfo) Unmarshal() error {
	app.PagesMap = make(map[string]*Page)
	bytes, err := ioutil.ReadFile(filepath.Join(RootDir, SourceDir, "app.json"))
	if err != nil {
		return err
	}
	err = json.Unmarshal(bytes, app)
	if err != nil {
		return err
	}
	for i, p := range app.Pages {
		if SourceDir != "" {
			p = SourceDir + "/" + p
			app.Pages[i] = p
		}
		var page Page
		if err := page.Unmarshal(p); err != nil {
			continue
		}
		app.PagesMap[page.Path] = &page
	}
	for _, sp := range app.SubPages {
		for i, p := range sp.Pages {
			if SourceDir != "" {
				p = SourceDir + "/" + p
				sp.Pages[i] = p
			}
			var page Page
			err := page.Unmarshal(filepath.Join(sp.Root, p))
			if err != nil {
				continue
			}
			app.PagesMap[page.Path] = &page
		}
	}
	return nil
}
func (app *AppInfo) HasPage(path string) bool {
	_, ok := app.PagesMap[path]
	return ok
}
