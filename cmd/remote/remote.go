package main

import (
	"image"
	"log"
	"os"
	"time"

	"github.com/frizinak/autodroid/adb"
)

func main() {
	shot := adb.New("adb", 1024*1024*30)
	if err := shot.Init(); err != nil {
		panic(err)
	}
	defer shot.Close()
	input := adb.New("adb", 1024*1024)
	if err := input.Init(); err != nil {
		panic(err)
	}
	defer input.Close()

	app := New(input, log.New(os.Stderr, "", 0))
	imgs := make(chan *image.NRGBA, 2)
	go func() {
		for {
			img, err := shot.Screencap()
			if err != nil {
				log.Println(err)
			}
			if img != nil {
				imgs <- img
			}
			time.Sleep(time.Millisecond * 32)
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
