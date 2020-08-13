package parse

import (
	"log"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/tdewolff/parse/v2/js"
)

type JsGenerator interface {
	StartWriteConfig()  //生成配置项即App() Page() Component()中的配置
	EndWriteConfig()    //结束配置项生成
	Write(string)       //普通数据
	WriteImport(string) //生成require("xxx.wxs")
	StartWriteBehavior()
	EndWriteBehavior()
}
type JsParser struct {
	Path string
}

func (p *JsParser) ParseJs(g JsGenerator) {
	input, err := os.Open(filepath.Join(RootDir, p.Path))
	if err != nil {
		log.Print(err)
		return
	}
	var brackets int
	var startImport bool
	var keyIdent string
	var startKey bool
	var startBehavior bool
	lexer := js.NewLexer(input)
	for t, b := lexer.Next(); t != js.ErrorToken; t, b = lexer.Next() {
		str := *(*string)(unsafe.Pointer(&b))
		if keyIdent != "" {
			if b[0] == '(' {
				startKey = true
			} else {
				g.Write(keyIdent)
			}
			keyIdent = ""
		}
		switch t {
		//case js.SingleLineCommentToken, js.MultiLineCommentToken:
		//	continue
		case js.IdentifierToken:
			switch str {
			case "require", "import":
				startImport = true
			case "App", "Page", "Component":
				keyIdent = str
				continue
			case "Behavior":
				startBehavior = true
				continue
			default:
			}
		case js.PunctuatorToken:
			if startKey {
				switch b[0] {
				case '(':
					if brackets++; brackets == 1 {
						g.StartWriteConfig()
						continue
					}
				case ')':
					if brackets--; brackets == 0 {
						startKey = false
						g.EndWriteConfig()
						continue
					}
				}
			} else if startBehavior {
				switch b[0] {
				case '(':
					if brackets++; brackets == 1 {
						g.StartWriteBehavior()
						continue
					}
				case ')':
					if brackets--; brackets == 0 {
						startBehavior = false
						g.EndWriteBehavior()
						continue
					}
				}
			}
		case js.StringToken:
			if startImport {
				if str[1] == '.' {
					g.WriteImport(rb(str))
				}
				startImport = false
			}
		}
		g.Write(str)
	}
}
