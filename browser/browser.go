package browser

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"lizhanpeng.org/lizhanpeng/agent/controller"
)

// browser state
type BrowserState struct {
	DomState
	Url         string
	Title       string
	Tabs        []*TabInfo
	ScreentShot []byte
	PixelsAbove int
	PixelBelow  int
}

type TabInfo struct {
	TargetId string
	PageId   int
	Url      string
	Title    string
}

// one browser
type Browser struct {
	DomService  *DomService
	ctx         context.Context   // root
	current     context.Context   // current
	tabs        []context.Context // recording all tabs, order by insert timestamp
	CachedState *BrowserState     // get state in a loop
}

func NewBrowser() *Browser {
	b := new(Browser)
	b.DomService = NewDomService(b)
	return b
}

func (b *Browser) newChromeDpContext() context.Context {
	parent := b.ctx
	if b.ctx == nil {
		opts := chromedp.DefaultExecAllocatorOptions[3:]
		opts = append(opts, chromedp.NoFirstRun, chromedp.NoDefaultBrowserCheck)
		ctx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
		parent = ctx
	}
	ctx, _ := chromedp.NewContext(parent)
	if b.ctx == nil {
		b.ctx = ctx
	}
	b.current = ctx
	b.tabs = append(b.tabs, ctx)
	return ctx
}

func (b *Browser) getCurrentPage() context.Context {
	if b.ctx == nil {
		b.newChromeDpContext()
	}
	return b.current
}

func (b *Browser) GetState() *BrowserState {
	if b.CachedState == nil {
		panic("not siupposed to be here")
	}
	return b.CachedState
}

func (b *Browser) UpdateState() {
	b.DomService.RemoveHightLights()
	domState := b.DomService.GetClickableElements()
	screentShot := b.Screenshot()
	scrollAbove, scrollBelow := b.GetScrollInfo()
	tabs, tab := b.getTabsInfo()
	state := &BrowserState{
		DomState:    *domState,
		Url:         tab.Url,
		Title:       tab.Title,
		Tabs:        tabs,
		ScreentShot: screentShot,
		PixelsAbove: scrollAbove,
		PixelBelow:  scrollBelow,
	}
	b.CachedState = state
}

func (b *Browser) getSelectorMap() SelectorMap {
	if b.CachedState == nil {
		return nil
	}
	return b.CachedState.SelectorMap
}

func (b *Browser) getTabsInfo() ([]*TabInfo, *TabInfo) {
	ret := make([]*TabInfo, 0)
	tabInfos, err := chromedp.Targets(b.current)
	if err != nil {
		panic(err)
	}
	mp := make(map[string]*TabInfo)
	for _, tab := range tabInfos {
		if tab.Type != "page" {
			continue
		}
		mp[string(tab.TargetID)] = &TabInfo{
			TargetId: string(tab.TargetID),
			Url:      tab.URL,
			Title:    tab.Title,
		}
	}
	var currentTabInfo *TabInfo
	currentId := chromedp.FromContext(b.current).Target.TargetID.String()
	for i, tab := range b.tabs {
		id := chromedp.FromContext(tab).Target.TargetID.String()
		if mp[id] == nil {
			panic("tab not found")
		}
		info := &TabInfo{
			TargetId: mp[id].TargetId,
			PageId:   i,
			Url:      mp[id].Url,
			Title:    mp[id].Title,
		}
		ret = append(ret, info)
		if id == currentId {
			currentTabInfo = info
		}
	}
	return ret, currentTabInfo
}

func (b *Browser) getChromeDpTabs() map[string]*TabInfo {
	ret := make(map[string]*TabInfo)
	tabInfos, err := chromedp.Targets(b.current)
	if err != nil {
		panic(err)
	}
	for _, tab := range tabInfos {
		if tab.Type != "page" {
			continue
		}
		info := &TabInfo{
			TargetId: string(tab.TargetID),
			Url:      tab.URL,
			Title:    tab.Title,
		}
		ret[string(tab.TargetID)] = info
	}
	return ret
}

func (b *Browser) newPage() context.Context {
	return b.newChromeDpContext()
}

// TODO: how to pass 「im not a robot」testing
func (b *Browser) GoogleSearch(param *GoogleSearchActionParam) {
	ctx := b.getCurrentPage()
	tasks := chromedp.Tasks{
		chromedp.Navigate(fmt.Sprint("https://www.google.com/search?q=%s&udm=14", param.Query)),
	}
	chromedp.Run(ctx, tasks...)
}

func (b *Browser) GoToUrlInCurrentTab(param *GoToUrlInCurrentTabParam) {
	ctx := b.getCurrentPage()
	tasks := chromedp.Tasks{
		chromedp.Navigate(param.Url),
	}
	chromedp.Run(ctx, tasks...)
}

func (b *Browser) GoToUelrlNewTab(param *GoToUrlNewTabParam) {
	ctx := b.newPage()
	tasks := chromedp.Tasks{
		chromedp.Navigate(param.Url),
	}
	chromedp.Run(ctx, tasks...)
}

func (b *Browser) GoBackward() {
	ctx := b.getCurrentPage()
	tasks := chromedp.Tasks{
		chromedp.NavigateBack(),
	}
	chromedp.Run(ctx, tasks...)
}

func (b *Browser) GoForward() {
	ctx := b.getCurrentPage()
	tasks := chromedp.Tasks{
		chromedp.NavigateForward(),
	}
	chromedp.Run(ctx, tasks...)
}

func (b *Browser) CloseCurrentTab() {
	ctx := b.getCurrentPage()
	tasks := chromedp.Tasks{
		page.Close(),
	}
	pageId := chromedp.FromContext(ctx).Target.TargetID.String()
	pageIndex := -1
	for i, tab := range b.tabs {
		target := chromedp.FromContext(tab).Target
		if target == nil {
			continue
		}
		if target.TargetID.String() == pageId {
			pageIndex = i
			break
		}
	}
	if pageIndex >= 0 {
		var tabs []context.Context
		if pageIndex > 0 {
			tabs = append(tabs, b.tabs[:pageIndex]...)
		}
		if pageIndex < len(b.tabs)-1 {
			tabs = append(tabs, b.tabs[pageIndex+1:]...)
		}
	}
	chromedp.Run(ctx, tasks...)
	b.SwithTab(&SwitchTabParam{
		PageIndex: 0,
	})
}

func (b *Browser) SwithTab(param *SwitchTabParam) {
	if param.PageIndex >= len(b.tabs) {
		return
	} else if param.PageIndex < -1 {
		return
	}
	var ctx context.Context
	if param.PageIndex == -1 {
		ctx = b.tabs[len(b.tabs)-1]
	} else {
		ctx = b.tabs[param.PageIndex]
	}
	b.current = ctx
	tasks := chromedp.Tasks{
		page.BringToFront(),
	}
	chromedp.Run(ctx, tasks...)
}

func (b *Browser) Screenshot() []byte {
	var out []byte
	ctx := b.getCurrentPage()
	tasks := chromedp.Tasks{
		chromedp.FullScreenshot(&out, 90),
	}
	chromedp.Run(ctx, tasks...)
	// os.WriteFile("/tmp/snap.jpeg", out, 0777)
	return out
}

func (b *Browser) ExecJavascript(param *ExecJavascriptParam) []byte {
	var out []byte
	var valOut []byte
	ctx := b.getCurrentPage()
	tasks := chromedp.Tasks{
		chromedp.Evaluate("1+1", &valOut),
		chromedp.Evaluate(param.Content, &out), // execute js to highlight elements
	}
	chromedp.Run(ctx, tasks...)
	if string(valOut) != "2" {
		panic("javascript not work")
	}
	return out
}

func (b *Browser) Wait(param *WaitScondsParam) {
	time.Sleep(time.Duration(param.Seconds) * time.Second)
}

func (b *Browser) GetClickElements() *DomState {
	return b.DomService.GetClickableElements()
}

func (b *Browser) GetScrollInfo() (int, int) {
	scrollYStr := b.ExecJavascript(&ExecJavascriptParam{
		Content: "window.scrollY",
	})
	viewPortHeightStr := b.ExecJavascript(&ExecJavascriptParam{
		Content: "window.innerHeight",
	})
	totalHeightStr := b.ExecJavascript(&ExecJavascriptParam{
		Content: "document.documentElement.scrollHeight",
	})
	scrollY, _ := strconv.Atoi(string(scrollYStr))
	viewPortHeight, _ := strconv.Atoi(string(viewPortHeightStr))
	totalHeight, _ := strconv.Atoi(string(totalHeightStr))
	return scrollY, totalHeight - (scrollY + viewPortHeight)
}

func (b *Browser) ClickElement(param *ClickElementParam) {
	smp := b.getSelectorMap()
	if smp == nil {
		return
	}
	node := smp[param.Index]
	if node == nil {
		return
	}
	ctx := b.getCurrentPage()
	tasks := chromedp.Tasks{
		// chromedp.ScrollIntoView(node.XPath, chromedp.BySearch),
		// will wait until is visable
		chromedp.Click(node.XPath, chromedp.BySearch),
	}
	chromedp.Run(ctx, tasks...)
	// todo 跳转到新的tab？
	tabs := b.getChromeDpTabs()
	if len(tabs) == len(b.tabs) {
		// no new tabs opened
		// pass
	} else if len(tabs) == len(b.tabs)+1 {
		// open new tab
		for _, c := range b.tabs {
			id := chromedp.FromContext(c).Target.TargetID.String()
			delete(tabs, id)
		}
		if len(tabs) != 1 {
			panic("not supported to be here")
		}
		for id, _ := range tabs {
			nCtx, _ := chromedp.NewContext(b.ctx, chromedp.WithTargetID(target.ID(id)))
			b.tabs = append(b.tabs, nCtx)
		}
		b.SwithTab(&SwitchTabParam{
			PageIndex: -1,
		})
	} else {
		panic("not supposed to be here")
	}
}

func (b *Browser) InputText(param *InputTextParam) {
	smp := b.getSelectorMap()
	if smp == nil {
		return
	}
	node := smp[param.Index]
	if node == nil {
		return
	}
	ctx := b.getCurrentPage()
	tasks := chromedp.Tasks{
		chromedp.SendKeys(node.XPath, param.Input, chromedp.BySearch),
	}
	chromedp.Run(ctx, tasks...)
}

// input parameters
type GoogleSearchActionParam struct {
	Query string
}

type GoToUrlInCurrentTabParam struct {
	Url string
}

type GoToUrlNewTabParam struct {
	Url string
}

type SwitchTabParam struct {
	PageIndex int
}

type ExecJavascriptParam struct {
	Content string
}

type WaitScondsParam struct {
	Seconds int
}

type ClickElementParam struct {
	Index int
}
type InputTextParam struct {
	Index int
	Input string
}

func init() {
	controller.RegistryAction("wait", "Wait for x seconds default 3", new(WaitScondsParam))
	controller.RegistryAction("search_google", "'Search the query in Google in the current tab, the query should be a search query like humans search in Google, concrete and not vague or super long. More the single most important items.", new(GoogleSearchActionParam))
	controller.RegistryAction("go_to_url", "Navigate to URL in the current tab", new(GoToUrlInCurrentTabParam))
	controller.RegistryAction("go_back", "Go back", nil)
	controller.RegistryAction("go_forward", "Go Forward", nil)
	controller.RegistryAction("switch_tab", "Switch tab", new(SwitchTabParam))
	controller.RegistryAction("open_tab", "Open url in new tab", new(GoToUrlNewTabParam))
}
