package rn

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"github.com/langhuihui/ace/parse"
	"github.com/langhuihui/ace/util"
	. "github.com/langhuihui/ace/generator"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/tdewolff/parse/v2/css"
)

var (
	reg_important = regexp.MustCompile("!important")
	Directs       = [4]string{"Top", "Right", "Bottom", "Left"}
	AllImportCss  = make(util.HashSet)
	importCssLock sync.Mutex
)

func transFormValue(x string) string {
	if util.RegNumberic.MatchString(x) {
		return x
	}
	if strings.HasSuffix(x, "%") {
		return "0"
	} else {
		return "'" + x + "'"
	}
}
func valueNumberCheck(x string) string {
	if util.RegNumberic.MatchString(x) {
		return x
	}
	return "`" + x + "`"
}
func (r *ReactNative) GenerateCss() {
	if r.Generator.GenerateCss(r.Path) == nil {
		r.hasWxss = true
	}
}
func (g *Generator)GenerateCss(p string) error {
	input, err := os.Open(filepath.Join(parse.RootDir, p+".wxss"))
	if err != nil {
		if !os.IsNotExist(err) {
			log.Print(err)
		}
		return err
	}
	lexer := css.NewParser(input, false)
	atomic.AddInt32(&TotalCSSCount, 1)
	output, err := os.OpenFile(filepath.Join(OutputDir, p+".css.js"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0777)
	if err != nil {
		log.Print(err)
		return err
	}
	defer output.Close()
	fmt.Fprintf(output, `import ACECSS from "%s/ace-css.js"
const css = new ACECSS([ 
`, util.RelateivePath(filepath.Dir(p), "/utils"))
	defer output.WriteString("export default css")
	var inAtRule bool
	var imports []string
	var importants []string
	var ruleName []byte
	var isQualified bool
	var r []string
	var w []string
	var d strings.Builder
	readRuleName := func() {
		ruleName = nil
		var weights []string
		weight := 0
		var isClass bool
		for _, v := range lexer.Values() {
			ruleName = append(ruleName, v.Data...)
			switch v.TokenType {
			case css.WhitespaceToken:
				weights = append(weights, strconv.Itoa(weight))
				weight = 0
			case css.HashToken:
				weight += 1000000
			case css.DelimToken:
				switch v.Data[0] {
				case '.':
					isClass = true
					weight += 10000
				case '>':
					weights = append(weights, strconv.Itoa(weight))
					weight = 0
				}
			case css.IdentToken:
				if !isClass {
					weight += 9000
				} else {
					isClass = false
				}
			}
		}
		weights = append(weights, strconv.Itoa(weight))
		w = append(w, fmt.Sprintf("[%s]", strings.Join(weights, ",")))
		r = append(r, fmt.Sprintf("`%s`", ruleName))
	}
	for gt, t, b := lexer.Next(); t != css.ErrorToken; gt, t, b = lexer.Next() {
		switch gt {
		case css.QualifiedRuleGrammar:
			if inAtRule {
				break
			}
			isQualified = true
			readRuleName()
		case css.AtRuleGrammar:
			if string(b) == "@import" {
				b = lexer.Values()[1].Data
				importSrc := util.RelateivePath(filepath.Dir(p), string(b[1:len(b)-1]))
				importSrc = strings.TrimSuffix(importSrc, ".wxss")
				importRealPath := filepath.Join(filepath.Dir(p), importSrc)
				importCssLock.Lock()
				if !AllImportCss.Has(importRealPath) {
					os.MkdirAll(filepath.Join(OutputDir, filepath.Dir(importRealPath)), 0777)
					go g.GenerateCss(importRealPath)
					AllImportCss.Add(importRealPath)
				}
				importCssLock.Unlock()
				imports = append(imports, importSrc)
			}
		case css.EndAtRuleGrammar:
			inAtRule = false
		case css.BeginAtRuleGrammar:
			inAtRule = true
		case css.BeginRulesetGrammar:
			if inAtRule {
				break
			}
			readRuleName()
		case css.EndRulesetGrammar:
			if inAtRule {
				break
			}
			if d.Len() > 0 {
				if isQualified {
					fmt.Fprintf(output, `{r:[%s],
w:[%s],
d:{%s},
`, strings.Join(r, ","), strings.Join(w, ","), d.String())
				} else {
					fmt.Fprintf(output, `{r:%s,
w:%s,
d:{%s},
`, r[0], w[0], d.String())
				}
				if len(importants) > 0 {
					fmt.Fprintf(output, "i:[%s]", strings.Join(importants, ","))
				}
				output.WriteString("},\n")
			}
			d.Reset()
			isQualified = false
			importants = nil
			r = nil
			w = nil
		case css.DeclarationGrammar:
			output := &d
			if inAtRule {
				break
			}
			values := lexer.Values()
			if len(values) == 0 {
				log.Printf("%s.wxss:1:0 样式属性没有值：%s: %s", filepath.Join(parse.RootDir, p), ruleName, b)
				break
			}
			propName := string(b)
			rnPropName := util.CamelCase(strings.Split(propName, "-"))
			switch propName {
			case "box-sizing", "float", "overflow-x", "overflow-y", "white-space", "text-overflow", "animation", "transition", "background-image", "background-size":
			case "margin", "padding", "border-width":
				var properties [][]byte
				var important bool
				for _, v := range values {
					switch v.TokenType {
					case css.IdentToken, css.NumberToken, css.DimensionToken, css.PercentageToken:
						properties = append(properties, v.Data)
					case css.DelimToken:
						if v.Data[0] == '!' {
							important = true
							break
						}
					}
				}
				switch len(properties) {
				case 1:
					properties = append(properties, properties...)
					fallthrough
				case 2:
					properties = append(properties, properties...)
					fallthrough
				case 3:
					if len(properties) == 3 {
						properties = append(properties, properties[2])
					}
					fallthrough
				case 4:
					for i, d := range Directs {
						if propName == "border-width" {
							d = "border" + d + "Width"
						} else {
							d = propName + d
						}
						if util.RegNumberic.Match(properties[i]) {
							fmt.Fprintf(output, "\n\t%s:%s,", d, properties[i])
						} else {
							fmt.Fprintf(output, "\n\t%s:'%s',", d, properties[i])
						}
						if important {
							importants = append(importants, "'"+d+"'")
						}
					}
				}
			case "border-top-style", "border-bottom-style", "border-left-style", "border-right-style":
				for _, v := range values {
					if v.TokenType == css.IdentToken {
						fmt.Fprintf(output, "\n\tborderStyle:'%s',", v.Data)
					}
				}
			case "border-top", "border-bottom", "border-left", "border-right":
				inFunction := false
				for _, v := range values {
					if inFunction {
						output.Write(v.Data)
						if v.TokenType == css.RightParenthesisToken {
							inFunction = false
							output.WriteString("',")
						}
						continue
					}
					switch v.TokenType {
					case css.DelimToken:
						if v.Data[0] == '!' {
							importants = append(importants, "'"+rnPropName+"'")
							break
						}
					case css.IdentToken:
						fmt.Fprintf(output, "\n\tborderStyle:'%s',", v.Data)
					case css.HashToken:
						fmt.Fprintf(output, "\n\t%sColor:'%s',", rnPropName, v.Data)
					case css.NumberToken:
						fmt.Fprintf(output, "\n\t%sWidth:%s,", rnPropName, v.Data)
					case css.DimensionToken, css.PercentageToken:
						fmt.Fprintf(output, "\n\t%sWidth:'%s',", rnPropName, v.Data)
					case css.FunctionToken:
						inFunction = true
						fmt.Fprintf(output, "\n\t%sColor:'%s", rnPropName, v.Data)
					case css.RightBraceToken:
						output.WriteString("',")
					default:
						output.Write(v.Data)
					}
				}
			case "background":
				var bgColor []byte
				inFunction := false
				for _, v := range values {
					if inFunction {
						bgColor = append(bgColor, v.Data...)
						if v.TokenType == css.RightParenthesisToken {
							inFunction = false
						}
						continue
					}
					switch v.TokenType {
					case css.URLToken, css.WhitespaceToken:
					case css.FunctionToken:
						inFunction = true
						bgColor = append(bgColor, v.Data...)
					default:
						if v.Data[0] == '!' {
							importants = append(importants, "'"+rnPropName+"'")
							break
						}
					}
				}
				if bgColor != nil {
					fmt.Fprintf(output, "\n\tbackgroundColor:'%s',", bgColor)
				}
			case "transform":
				var propsName string
				var value strings.Builder
				var props []string
				for _, v := range values {
					switch v.TokenType {
					case css.FunctionToken:
						propsName = string(v.Data[:len(v.Data)-1])
						value.Reset()
					case css.RightParenthesisToken:
						valueStr := value.String()
						xy := strings.Split(valueStr, ",")
						if len(xy) > 1 {
							props = append(props, fmt.Sprintf("{%sX:%s}", propsName, transFormValue(xy[0])), fmt.Sprintf("{%sY:%s}", propName, transFormValue(xy[1])))
						} else {
							props = append(props, fmt.Sprintf("{%s:%s}", propsName, transFormValue(valueStr)))
						}
					default:
						value.Write(v.Data)
					}
				}
				fmt.Fprintf(output, "\n\ttransform:[%s],", strings.Join(props, ","))
			case "text-decoration":
				rnPropName = "textDecorationLine"
				fallthrough
			default:
				if strings.HasPrefix(propName, "-webkit") {
					break
				}
				var value strings.Builder
				for _, v := range values {
					if v.Data[0] == '!' {
						importants = append(importants, "'"+rnPropName+"'")
						break
					} else if v.Data[0] == '\'' || v.Data[0] == '"' {
						continue
					}
					value.Write(v.Data)
				}
				str := value.String()
				if util.RegNumberic.MatchString(str) {
					fmt.Fprintf(output, "\n\t%s:%s,", rnPropName, str)
				} else {
					fmt.Fprintf(output, "\n\t%s:`%s`,", rnPropName, str)
				}
			}
		}
	}
	output.Seek(-2, 1)
	output.WriteString(`])
`)
	if len(imports) > 0 {
		for k, v := range imports {
			imports[k] = fmt.Sprintf("require('%s.css.js').default", v)
		}
		fmt.Fprintf(output, "css.merge(%s)\n", strings.Join(imports, ","))
	}
	return nil
}
