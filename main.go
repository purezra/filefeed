package main

import (
	"log"

	"github.com/egoist/mygo"
)

func main() {
	mygo.Bind(Service{})

	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:           "FileFeed · 新增文件看板",
			TitleBarStyle:   mygo.TitleBarHidden,
			TitleBarHeight:  40,
			Width:           1280,
			Height:          820,
			MinWidth:        720,
			MinHeight:       480,
			BackgroundColor: "light-dark(#f4f6f9, #0e1014)",
			URL:             "/",
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
