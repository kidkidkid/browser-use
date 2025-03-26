package main

import (
	"lizhanpeng.org/lizhanpeng/agent/browser"
)

// 打开新标签页
// 获得所有新标签页
// 如何跳转标签页
// 如何点击

func main() {
	b := browser.NewBrowser()
	// b.GoogleSearch(&browser.GoogleSearchAction{
	// 	Query: "今天上海的天气",
	// })
	b.GoToUrlInCurrentTab(&browser.GoToUrlInCurrentTabParam{
		Url: "file:///private/tmp/test.html",
	})
	b.Wait(&browser.WaitScondsParam{
		Seconds: 1,
	})
	b.DomService.RemoveHightLights()
	b.UpdateState()
	for id, node := range b.GetState().DomState.SelectorMap {
		// if node.Attributes["href"] == "" {
		// 	continue
		// }
		// b.ClickElement(&browser.ClickElementParam{
		// 	Index: id,
		// })
		if node.TagName == "input" {
			b.InputText(&browser.InputTextParam{
				Index: id,
				Input: "123",
			})
		}
		return
	}
	b.Wait(&browser.WaitScondsParam{
		Seconds: 2,
	})
	b.GoBackward()
	b.Wait(&browser.WaitScondsParam{
		Seconds: 2,
	})
	b.GoForward()
	b.GoToUrlInCurrentTab(&browser.GoToUrlInCurrentTabParam{
		Url: "http://www.baidu.com/ss",
	})
	b.CloseCurrentTab()
	b.GoToUrlInCurrentTab(&browser.GoToUrlInCurrentTabParam{
		Url: "http://www.baidu.com/sss",
	})
}
