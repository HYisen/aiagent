package main

import (
	"aiagent/console"
	"aiagent/tools/client/clients/ai"
	"aiagent/tools/client/ui"
	"flag"
)

var endpoint = flag.String("endpoint", "https://hyisen.net/ai", "aiagent endpoint")
var softWrapWidth = flag.Int("softWrapWidth", 0, "soft wrap output line at width for unable terminals, 0 for disable")
var wideCharScale = flag.Float64("wideCharScale", 1.667, "one CJK wide char as how many ASCII chars in soft wrap")

func main() {
	flag.Parse()
	handler := ui.NewChatLineHandler(ai.NewClient(*endpoint, ui.Login), ui.SoftWrapOptions{
		TerminalWidth: *softWrapWidth,
		WideCharScale: *wideCharScale,
	})
	controller := console.NewController(handler, console.NewDefaultOptions())
	controller.Run()
}
