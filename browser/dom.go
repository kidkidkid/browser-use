package browser

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type DomService struct {
	Browser *Browser
}

type DomState struct {
	ElemmentTree *DomElementNode
	SelectorMap  SelectorMap
}

func NewDomService(b *Browser) *DomService {
	d := new(DomService)
	d.Browser = b
	return d
}

type domJsParam struct {
	DoHighlightElements bool `json:"doHighlightElements"`
	FocusHighlightIndex int  `json:"focusHighlightIndex"`
	ViewportExpansion   int  `json:"viewportExpansion"`
	DebugMode           bool `json:"debugMode"`
}

func (p *domJsParam) String() string {
	d, _ := json.Marshal(p)
	return string(d)
}

func (d *DomService) AddHightlights() []byte {
	param := &domJsParam{
		DoHighlightElements: true,
		FocusHighlightIndex: -1,
		ViewportExpansion:   0,
		DebugMode:           false,
	}
	content := fmt.Sprintf(`(%s)(%s)`, domJs, param.String())
	out := d.Browser.ExecJavascript(&ExecJavascriptParam{
		Content: content,
	})
	return out
}

func (d *DomService) RemoveHightLights() {
	_ = d.Browser.ExecJavascript(&ExecJavascriptParam{
		Content: removeHighlightJs,
	})
}

func (d *DomService) GetClickableElements() *DomState {
	out := d.AddHightlights()
	rootNode, sMap := ConstructDomTree(out)
	return &DomState{
		ElemmentTree: rootNode,
		SelectorMap:  sMap,
	}
}

type Coordinates struct {
	X int
	Y int
}

type CoordinateSet struct {
	TopLeft     Coordinates
	TopRight    Coordinates
	BottomLeft  Coordinates
	BottomRight Coordinates
	Center      Coordinates
	Width       int
	Height      int
}

type ViewportInfo struct {
	ScrollX int
	ScrollY int
	Width   int
	Height  int
}

type DomNodeI interface {
	SetParent(*DomElementNode)
}

type DomNode struct {
	IsVisvable bool `json:"isVisible"`
	Parent     *DomElementNode
}

func (d *DomNode) SetParent(n *DomElementNode) {
	d.Parent = n
}

type DomTextNode struct {
	Text string `json:"text"`
	DomNode
}

func (d *DomTextNode) hasParentWithHighlight() bool {
	current := d.Parent
	for current != nil {
		if current.HighlightIndex != nil {
			return true
		}
		current = current.Parent
	}
	return false
}

type DomElementNode struct {
	TagName             string            `json:"tagName"`
	XPath               string            `json:"xpath"`
	Attributes          map[string]string `json:"attributes"`
	IsInteractive       bool              `json:"isVisible"`
	IsTopElement        bool              `json:"isTopElement"`
	IsInViewPoint       bool              `json:"isInViewport"`
	ShadowRoot          bool              `json:"shadowRoot"`
	HighlightIndex      *int              `json:"highlightIndex"`
	ViewportCoordinates CoordinateSet     `json:"-"`
	PageCoordinates     CoordinateSet     `json:"-"`
	ViewportInfo        ViewportInfo      `json:"viewport"`
	ChildrenIDs         []string          `json:"children"`
	Childrens           []DomNodeI
	DomNode
}

type DomTree struct {
	RootId int                 `json:"rootId"`
	Map    map[string]DomNodeI `json:"map"`
}

type SelectorMap = map[int]*DomElementNode

func (d *DomElementNode) GetCliableElementsString() string {
	includeAttributes := map[string]bool{
		"title":         true,
		"type":          true,
		"name":          true,
		"role":          true,
		"tabindex":      true,
		"aria-label":    true,
		"placeholder":   true,
		"value":         true,
		"alt":           true,
		"aria-expanded": true,
	}
	formatedStrs := make([]string, 0)
	var processFn func(node DomNodeI, depth int)
	processFn = func(node DomNodeI, depth int) {
		if eNode, ok := node.(*DomElementNode); ok {
			if eNode.HighlightIndex != nil {
				attributes := make([]string, 0)
				text := d.getAllTextTillNextClickableElement(-1)
				for key, val := range eNode.Attributes {
					if !includeAttributes[key] {
						continue
					} else if val == eNode.TagName {
						continue
					}
					attributes = append(attributes, val)
				}
				line := fmt.Sprintf("[%d]<%s ", *eNode.HighlightIndex, eNode.TagName)
				attributesStr := strings.Join(attributes, ";")
				if attributesStr != "" {
					line += fmt.Sprintf("{%s}", attributesStr)
				}
				if text != "" {
					if attributesStr != "" {
						line += fmt.Sprintf(">%s", text)
					} else {
						line += text
					}
				}
				line += "/>"
				formatedStrs = append(formatedStrs, line)
			}
			for _, child := range eNode.Childrens {
				processFn(child, depth+1)
			}
		} else if tNode, ok := node.(*DomTextNode); ok {
			if tNode.IsVisvable && !tNode.hasParentWithHighlight() {
				formatedStrs = append(formatedStrs, tNode.Text)
			}
		}
	}
	processFn(d, 0)
	return strings.Join(formatedStrs, "\n")
}

func (d *DomElementNode) getAllTextTillNextClickableElement(maxDepth int) string {
	textParts := make([]string, 0)
	var collectFn func(node DomNodeI, depth int)
	collectFn = func(node DomNodeI, depth int) {
		if maxDepth != -1 && depth > maxDepth {
			return
		}
		if eNode, ok := node.(*DomElementNode); ok {
			if eNode != d && eNode.HighlightIndex != nil {
				return
			}
			for _, child := range eNode.Childrens {
				collectFn(child, depth+1)
			}
		} else if tNode, ok := node.(*DomTextNode); ok {
			textParts = append(textParts, tNode.Text)
		}
	}
	collectFn(d, 0)
	return strings.Trim(strings.Join(textParts, "\n"), " ")
}

func ParseDomNode(mp map[string]any) DomNodeI {
	if mp == nil {
		return nil
	}
	data, err := json.Marshal(mp)
	if err != nil {
		panic(err)
	}
	var out DomNodeI
	if mp["type"] == "TEXT_NODE" {
		out = new(DomTextNode)
	} else {
		out = new(DomElementNode)
	}
	if err := json.Unmarshal(data, out); err != nil {
		panic(err)
	}
	return out
}

func ConstructDomTree(data []byte) (*DomElementNode, SelectorMap) {
	if len(data) == 0 {
		return nil, nil
	}
	mp := make(map[string]any)
	if err := json.Unmarshal(data, &mp); err != nil {
		panic(err)
	}
	selectorMap := make(SelectorMap)
	nodeMap := make(map[string]DomNodeI)
	for id, data := range mp["map"].(map[string]any) {
		node := ParseDomNode(data.(map[string]any))
		nodeMap[id] = node
		if _, ok := node.(*DomTextNode); ok {
			// pass
		} else if elementNode, ok := node.(*DomElementNode); ok {
			if elementNode.HighlightIndex != nil {
				selectorMap[*elementNode.HighlightIndex] = elementNode
			}
			for _, childId := range elementNode.ChildrenIDs {
				if nodeMap[childId] == nil {
					continue
				}
				childNode := nodeMap[childId]
				childNode.SetParent(elementNode)
				elementNode.Childrens = append(elementNode.Childrens, childNode)
			}
		} else {
			panic("not supposed to be here")
		}
	}
	var rootNode *DomElementNode
	rootId, ok := mp["rootId"].(string)
	if ok {
		n, ok := nodeMap[rootId].(*DomElementNode)
		if ok {
			rootNode = n
		}
	}
	for i, node := range nodeMap {
		if tNode, ok := node.(*DomTextNode); ok {
			fmt.Println(i, tNode.IsVisvable, tNode.Text)
		} else if eNode, ok := node.(*DomElementNode); ok {
			fmt.Println(i, *eNode)
		}
	}
	return rootNode, selectorMap
}

var domJs string
var removeHighlightJs = `try {
                    // Remove the highlight container and all its contents
                    const container = document.getElementById('playwright-highlight-container');
                    if (container) {
                        container.remove();
                    }

                    // Remove highlight attributes from elements
                    const highlightedElements = document.querySelectorAll('[browser-user-highlight-id^="playwright-highlight-"]');
                    highlightedElements.forEach(el => {
                        el.removeAttribute('browser-user-highlight-id');
                    });
                } catch (e) {
                    console.error('Failed to remove highlights:', e);
                }`

func init() {
	jsScript, err := os.ReadFile("./browser/buildDomTree.js")
	if err != nil {
		panic(err)
	}
	domJs = string(jsScript)
}
