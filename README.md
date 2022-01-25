# ADB Shell and simple android automation library


Shell:

```
import "github.com/frizinak/autodroid/adb"

client := adb.New("adb", 1024*1024*30)
if err := client.Init(); err != nil {
    panic(err)
}
defer client.Close()

var img *image.NRGBA
img, err = client.Screencap()

if err = client.SetBrightness(1000); err != nil {
    panic(err)
}

client.Tap(500, 300)
client.Drag(500, 300, 700, 300, time.Millisecond*300)

...
```

Automation:

```
import "github.com/frizinak/autodroid/adb"
import "github.com/frizinak/autodroid/auto"

...

icon, _ := png.Decode(...)
testA := auto.NewSubImgTest("test-chrome-app", icon, 5, image.Rect(50, 800, 750, 1080))

behaviors := auto.NewBehaviors(
    auto.Behavior{
        Tests: []auto.Test{
            testA,
            auto.NewOrTest(
                testB,
                testC,
            ),
        },
        Run: func(state *auto.State, s *auto.ImageSearch, results []auto.Result) error {
            fmt.Println("tests evaluated to true")
            state.Stop() // don't test/run anything else this iteration

            r := results[0] // result of testA
            client.Tap(r.Min.X, r.Min.Y)
        },
    },
    ...,
)

search := auto.NewImageSearch()
for {
    img, err := client.Screencap()
    if err != nil {
        panic(err)
    }
    search.Set(img)

    if err := behaviors.Do(search); err != nil {
        log.Println(err)
    }
}
```

## Examples

see cmd/remote
