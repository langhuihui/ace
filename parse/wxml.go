package parse

import (
	"io"
	"regexp"
	"strings"

	"github.com/langhuihui/ace/util"

	//"github.com/tdewolff/parse/v2/html"
	"github.com/tdewolff/parse/v2/js"
)

type WxmlFor struct {
	For   string
	Item  string
	Index string
}
type WxmlIf struct {
	Condition int
	Value     string
}
type Attribute struct {
	Raw       string //原始数据
	Value     string //去掉头尾引号的值
	JsxValue  string //用于放到Jsx中使用
	IsPureStr bool
}
type WxmlEl struct {
	TagName       string
	TargetTagName string
	Attrs         map[string]*Attribute
	Parent        *WxmlEl
	Template      *WxmlEl //在模板定义中
	Root          *WxmlEl //根元素
	Children      []*WxmlEl
	Text          string
	Line          int
	Column        int
	Depth         int
	ForDepth      int     //for循环的层级
	InFor         *WxmlEl //在For循环内部
	WxmlFor
	WxmlIf
	Previous        *WxmlEl      //前一个同级节点
	Next *WxmlEl//下一个同级节点
	Identifiers     util.HashSet //用到的js标识符
	HashCode        string
	CustomComponent bool //自定义组件
}

const (
	_ = iota
	COND_IF
	COND_ELIF
	COND_ELSE
	itoa62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

var reg_bracket = regexp.MustCompile("[+?:-]")

// Mustache 解析表达式
func (el *WxmlEl) Mustache(v0 string, quotation string, trim bool) (v string) {
	if quotation != "'" {
		quotation = `"`
	}
	s := strings.Split(v0, "{{")
	if len(s) > 1 {
		if trim {
			v = quotation + strings.TrimSpace(s[0]) + quotation
		} else {
			v = quotation + s[0] + quotation
		}
		for _, ss := range s[1:] {
			sss := strings.Split(ss, "}}")
			v += "+" + el.ParseJsId(sss[0])
			if len(sss[1]) > 0 {
				if trim {
					v += `+` + quotation + strings.TrimSpace(sss[1]) + quotation
				} else {
					v += `+` + quotation + sss[1] + quotation
				}
			}
		}
		v = strings.TrimPrefix(v, quotation+quotation+`+`)
	}
	return
}

// Bracket判断表达式是否需要添加括号
func Bracket(v string) string {
	if reg_bracket.MatchString(v) {
		return "(" + v + ")"
	}
	return v
}
func (el *WxmlEl) ParseJsId(jsstr string) string {
	var result strings.Builder
	var needBracket bool
	lexer := js.NewLexer(strings.NewReader(jsstr))
	var dot = false
	for t, b := lexer.Next(); t != js.ErrorToken; t, b = lexer.Next() {
		switch t {
		case js.PunctuatorToken:
			switch b[0] {
			case '.':
				dot = true
			case '+', '?', ':', '-':
				needBracket = true
			}
		case js.IdentifierToken:
			if !dot {
				id := string(b)
				switch id {
				case "true", "false", "null":
				default:
					if el.InFor != nil && (el.InFor.Index == id || el.InFor.Item == id) {
						//过滤掉for循环用到的标识符
						break
					}
					//关键词替换
					switch id {
					case "eval", "package", "switch", "GS", "HC":
						result.WriteString("this.data." + id)
						dot = false
						continue
					}
					if el.Template != nil {
						el.Template.Identifiers.Add(id)
					} else {
						el.Root.Identifiers.Add(id)
					}
					el.Identifiers.Add(id)
				}
			}
			fallthrough
		default:
			dot = false
		}
		result.Write(b)
	}
	if needBracket {
		return "(" + result.String() + ")"
	}
	return result.String()
}
func (el *WxmlEl) CreateChild(name string, line, column, depth int) (child *WxmlEl) {
	var pre *WxmlEl //前一个同级节点
	currentCount := len(el.Children)
	if currentCount > 0 {
		pre = el.Children[currentCount-1]
	}
	var hash = itoa62[currentCount%62 : (currentCount%62)+1]
	if currentCount >= 62 {
		hash = hash + strings.Repeat("z", currentCount/62)
	}
	child = &WxmlEl{
		TagName:     name,
		Parent:      el,
		Line:        line,
		Column:      column,
		Depth:       depth,
		Attrs:       make(map[string]*Attribute),
		Identifiers: make(util.HashSet),
		Previous:    pre,
		InFor:       el.InFor,
		ForDepth:    el.ForDepth,
		HashCode:    el.HashCode + hash,
		Template:    el.Template,
		Root:el.Root,
	}
	if pre!=nil{
		pre.Next = child
	}
	el.Children = append(el.Children, child)
	return
}
func (el *WxmlEl) Parse(r io.Reader, enter, exit func(*WxmlEl)) {
	current := el
	lexer := NewLexer(r)
	line, column, depth := 1, 0, el.Depth+1

	for t, b := lexer.Next(); t != ErrorToken; t, b = lexer.Next() {
		str := string(b)
		if newLines := len(strings.Split(str, "\n")) - 1; newLines > 0 {
			line = line + newLines
			column = lexer.Offset() - len(b)
		}
		column := lexer.Offset() - column - len(b)
		switch t {
		case StartTagToken:
			current = current.CreateChild(str[1:], line, column, depth)

			depth++
		case AttributeToken:
			s := strings.SplitN(str, "=", 2)
			k := strings.TrimSpace(s[0])
			var av Attribute
			var v string
			if len(s) == 2 {
				av.Raw = strings.TrimSpace(s[1])
				v = rb(av.Raw) //去掉头尾引号
				if !util.RegNumberic.MatchString(v) {
					if vv := current.Mustache(v, av.Raw[:1], false); vv == "" {
						//纯字符串则复用原有表达式
						av.IsPureStr = true
					} else {
						v = vv
					}
				} else if util.RegNotNumberic.MatchString(v) {
					//0开头的不算纯数字
					av.IsPureStr = true
				}
				av.Value = v
				av.JsxValue = v
				if av.IsPureStr {
					av.JsxValue = av.Raw
				}
			}
			switch k {
			case "name":
				if current.TagName == "template" {
					current.Template = current
					//current.Identifiers = make(util.HashSet)
					current.HashCode = ""
				}
			case "wx:if", "wx-if":
				current.Condition, current.Value = COND_IF, v
			case "wx:elif", "wx-elif":
				current.Condition, current.Value = COND_ELIF, v
			case "wx:else", "wx-else":
				current.Condition, current.Value = COND_ELSE, v
			case "wx:for", "wx-for", "wx:for-items":
				current.For, current.InFor = v, current
				current.ForDepth++
				if current.Index == "" {
					current.Index = "index"
				}
				if current.Item == "" {
					current.Item = "item"
				}
			case "wx:for-index", "wx-for-index", "wx:index":
				current.Index = v
			case "wx:for-item", "wx-for-item":
				current.Item = v
			default:
				current.Attrs[k] = &av
			}
		case StartTagCloseToken:
			current.Children = make([]*WxmlEl, 0)
			enter(current)
		case StartTagVoidToken:
			enter(current)
			fallthrough
		case EndTagToken:
			exit(current)
			depth--
			current = current.Parent
		case TextToken:
			if current == nil {
				break
			} else if current.TagName == "wxs" {
				current.Text += str
				break
			}
			str = strings.Trim(strings.TrimSpace(str), " \r\n")
			if len(str) == 0 {
				break
			}
			child := current.CreateChild("", line, column, depth)
			if text := child.Mustache(str, "'", true); text != "" {
				str = "{" + text + "}"
			}
			child.Text = str
			current.Children = append(current.Children, child)
			enter(child)
		}
	}
	if lexer.Err() != nil && lexer.Err() != io.EOF {
		println(lexer.Err().Error())
	}
	return
}
func (el *WxmlEl) LastChild() *WxmlEl {
	if len(el.Children) > 0 {
		return el.Children[len(el.Children)-1]
	}
	return nil
}
func (el *WxmlEl) RemoveChild(child *WxmlEl) {
	for i, c := range el.Children {
		if c == child {
			el.Children = append(el.Children[:i], el.Children[i+1:]...)
			break
		}
	}
}
