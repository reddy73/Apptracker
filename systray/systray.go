package main

import (
	"fmt"
	"hackfromhome"
	"io/ioutil"
	"log"

	"github.com/getlantern/systray"
)

func main() {
	systray.Run(onReady, onExit)
	// systray.Run(onReady, onExit)
	defer systray.Quit()

}

func onReady() {
	systray.SetIcon(getIcon("assets/work.ico"))
	// systray.SetTemplateIcon(assets.Data, assets.Data)
	systray.SetTitle("Redifining WFH")
	systray.SetTooltip("Redifining WFH")
	enable := systray.AddMenuItem("Enable", "Runs the Tracker")
	disable := systray.AddMenuItem("Disable", "Diables the Tracker")
	quit := systray.AddMenuItem("Exit", "stops the tracker")

	go func() {
		for {
			select {
			case <-enable.ClickedCh:
				log.Println("Enabling the app")
				hackfromhome.Flag = true
				hackfromhome.StartTracking()
			case <-quit.ClickedCh:
				systray.Quit()
				// return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-disable.ClickedCh:
				hackfromhome.Flag = false
				log.Println("Disabling the app")
			case <-quit.ClickedCh:
				systray.Quit()
				// return
			}
		}
	}()
}

func onExit() {
	systray.Quit()
}

func getIcon(s string) []byte {
	log.Println(s)
	b, err := ioutil.ReadFile(s)
	log.Println("DEBUG")
	log.Println(b)
	if err != nil {
		fmt.Print(err)
	}
	return b
}
