package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"time"

	"github.com/frizinak/autodroid/adb"
)

const ADB = "adb"

func main() {
	var sleep float64
	var dev string
	flag.Float64Var(&sleep, "i", 0, "sleep interval in seconds (float)")
	flag.StringVar(&dev, "d", "", "device serial")
	flag.Parse()

	if dev == "" {
		devs, err := adb.Devices(ADB)
		if err != nil {
			panic(err)
		}
		if len(devs) > 1 {
			fmt.Println("multiple devices connected, use -d to select the correct one")
			for _, d := range devs {
				fmt.Println(d)
			}
			os.Exit(1)
		}
	}

	shot := adb.New(ADB, dev, 1024*1024*30)
	if err := shot.Init(); err != nil {
		panic(err)
	}
	defer shot.Close()
	input := adb.New(ADB, dev, 1024*1024)
	if err := input.Init(); err != nil {
		panic(err)
	}
	defer input.Close()

	app := New(input, log.New(os.Stderr, "", 0))
	imgs := make(chan *image.NRGBA, 2)
	go func() {
		for {
			err := shot.ScreencapContinuous(func(img *image.NRGBA) error {
				imgs <- img
				time.Sleep(time.Millisecond * time.Duration(sleep*1000))
				return nil
			})
			if err != nil {
				log.Println(err)
			}
			time.Sleep(time.Second)
		}
	}()

	go func() {
		for i := range imgs {
			app.Set(i)
		}
	}()

	if err := app.Run(); err != nil {
		log.Println(err)
	}
}
