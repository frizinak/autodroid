package main

import (
	"image"
	_ "image/jpeg"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/frizinak/autodroid/adb"
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

const (
	fs       = 4
	stride   = 4
	vertices = 4
)

type points [stride * vertices]float32

func buf(d *points, x0, y0, x1, y1 float32) {
	d[0] = x1
	d[1] = y1
	d[4] = x1
	d[5] = y0
	d[8] = x0
	d[9] = y0
	d[12] = x0
	d[13] = y1
	d[2], d[3] = 1, 1
	d[6], d[7] = 1, 0
	d[10], d[11] = 0, 0
	d[14], d[15] = 0, 1
}

func initialize() (*glfw.Window, error) {
	if err := glfw.Init(); err != nil {
		return nil, err
	}
	glfw.WindowHint(glfw.Resizable, glfw.True)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.DoubleBuffer, 1)
	window, err := glfw.CreateWindow(
		800,
		800,
		"adb remote",
		nil,
		nil,
	)
	if err != nil {
		return nil, err
	}

	window.MakeContextCurrent()
	return window, nil
}

type App struct {
	window                             *glfw.Window
	monitor                            *glfw.Monitor
	videoMode                          *glfw.VidMode
	windowX, windowY, windowW, windowH int

	realWidth, realHeight int

	fullscreen bool

	invalidateVAOs bool

	proj mgl32.Mat4

	gErr error

	log *log.Logger

	rw       sync.Mutex
	isUpdate bool
	img      *image.NRGBA
	bounds   image.Rectangle

	mouseDownTime time.Time
	mouseDown     bool
	mouseDownPos  FPoint
	cursorPos     FPoint

	adb *adb.ADB
}

type FPoint struct{ X, Y float64 }

func New(adb *adb.ADB, log *log.Logger) *App {
	return &App{log: log, adb: adb}
}

func (app *App) Set(img *image.NRGBA) {
	app.rw.Lock()
	app.isUpdate = true
	app.img = img
	app.bounds = img.Bounds()
	app.rw.Unlock()
}

func (app *App) get() (*image.NRGBA, bool) {
	app.rw.Lock()
	upd := app.isUpdate
	app.isUpdate = false
	img := app.img
	app.rw.Unlock()
	return img, upd
}

func (r *App) onText(w *glfw.Window, char rune) {
	r.adb.Text(string(char))
}

func (r *App) onMouseButton(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mod glfw.ModifierKey) {
	if button != glfw.MouseButton1 {
		return
	}
	if action == glfw.Press {
		r.mouseDown = true
		r.mouseDownTime = time.Now()
		r.mouseDownPos = r.cursorPos
	} else if action == glfw.Release {
		r.mouseDown = false
		t := r.TranslateCoords(r.cursorPos)
		since := time.Since(r.mouseDownTime)
		if since <= time.Millisecond*50 {
			if err := r.adb.Tap(t.X, t.Y); err != nil {
				r.log.Println(err)
			}
		} else {
			f := r.TranslateCoords(r.mouseDownPos)
			if err := r.adb.Drag(f.X, f.Y, t.X, t.Y, since); err != nil {
				r.log.Println(err)
			}
		}
	}
}

func (r *App) TranslateCoords(i FPoint) image.Point {
	dims := r.bounds
	rat := float64(dims.Dx()) / float64(dims.Dy())
	w, h := r.realWidth, int(float64(r.realWidth)/rat)
	if float64(r.realHeight)/float64(dims.Dx()) > float64(r.realWidth)/float64(dims.Dy()) {
		w, h = int(float64(r.realHeight)*rat), r.realHeight
	}

	xoffset := float64(r.realWidth-w) / 2
	yoffset := float64(r.realHeight-h) / 2
	scale := float64(dims.Dx()) / float64(w)

	return image.Point{int(scale * (i.X - xoffset)), int(scale * (i.Y - yoffset))}
}

func (r *App) onCursor(w *glfw.Window, x, y float64) {
	r.cursorPos.X, r.cursorPos.Y = x, y
}

func (r *App) onKey(w *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	if action == glfw.Release {
		return
	}

	if key == glfw.KeyQ && mods&glfw.ModControl != 0 {
		r.window.SetShouldClose(true)
	}
}

func (r *App) onResize(wnd *glfw.Window, width, height int) {
	r.realWidth, r.realHeight = width, height
	r.invalidateVAOs = true
	gl.Viewport(0, 0, int32(width), int32(height))
	r.proj = mgl32.Ortho2D(0, float32(width), float32(height), 0)
	if r.fullscreen {
		return
	}
	r.windowW, r.windowH = width, height
}
func (r *App) onPos(wnd *glfw.Window, x, y int) {
	if r.fullscreen {
		return
	}
	r.windowX, r.windowY = x, y
}

func (r *App) toggleFS() {
	r.fullscreen = !r.fullscreen
	if r.fullscreen {
		r.window.SetMonitor(r.monitor, 0, 0, r.videoMode.Width, r.videoMode.Height, r.videoMode.RefreshRate)
		return
	}
	r.window.SetMonitor(nil, r.windowX, r.windowY, r.windowW, r.windowH, r.videoMode.RefreshRate)
}

func (r *App) Run() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var err error
	r.window, err = initialize()
	defer glfw.Terminate()
	if err != nil {
		return err
	}
	r.monitor = glfw.GetPrimaryMonitor()
	r.videoMode = r.monitor.GetVideoMode()
	r.windowX, r.windowY = r.window.GetPos()
	r.windowW, r.windowH = r.window.GetSize()
	r.invalidateVAOs = false
	r.fullscreen = false
	r.proj = mgl32.Ortho2D(0, 800, 800, 0)

	if err := gl.Init(); err != nil {
		return err
	}

	r.window.SetFramebufferSizeCallback(r.onResize)
	r.window.SetPosCallback(r.onPos)
	r.window.SetKeyCallback(r.onKey)
	r.window.SetMouseButtonCallback(r.onMouseButton)
	r.window.SetCursorPosCallback(r.onCursor)
	r.window.SetCharCallback(r.onText)
	w, h := r.window.GetFramebufferSize()
	r.onResize(r.window, w, h)

	program, err := newProgram()
	if err != nil {
		return err
	}
	gl.UseProgram(program)
	gl.Enable(gl.TEXTURE_2D)

	textures := make([]uint32, 1)
	vaos := make([]uint32, 1)
	vbos := make([]uint32, 1)
	dimensions := make([]image.Point, 1)
	var tex uint32 = 0
	var vao uint32 = 0
	var dimension image.Point
	model := mgl32.Ident4()

	lastProjection := mgl32.Ident4()
	var lastTex uint32 = 0

	modelUniform := gl.GetUniformLocation(program, gl.Str("model\x00"))
	projectionUniform := gl.GetUniformLocation(program, gl.Str("projection\x00"))

	var ebo uint32
	indices := []uint32{0, 1, 3, 1, 2, 3}
	gl.GenBuffers(1, &ebo)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ebo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, 6*fs, gl.Ptr(indices), gl.STATIC_DRAW)

	newEntry := func(index int, bounds image.Rectangle) {
		if vaos[index] != 0 {
			return
		}

		d := points{}
		buf(&d, 0, 0, float32(bounds.Dx()), float32(bounds.Dy()))
		var vao, vbo uint32
		gl.GenVertexArrays(1, &vao)
		gl.GenBuffers(1, &vbo)

		gl.BindVertexArray(vao)

		gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ebo)
		gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
		gl.BufferData(gl.ARRAY_BUFFER, stride*vertices*fs, gl.Ptr(&d[0]), gl.DYNAMIC_DRAW)

		gl.EnableVertexAttribArray(0)
		gl.VertexAttribPointer(0, 2, gl.FLOAT, false, stride*fs, gl.PtrOffset(0))
		gl.EnableVertexAttribArray(1)
		gl.VertexAttribPointer(1, 2, gl.FLOAT, false, stride*fs, gl.PtrOffset(2*fs))

		gl.BindBuffer(gl.ARRAY_BUFFER, 0)
		gl.BindVertexArray(0)
		gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, 0)

		vaos[index] = vao + 1
		vbos[index] = vbo + 1
		dimensions[index] = image.Pt(bounds.Dx(), bounds.Dy())
	}

	getVAO := func(index int) (uint32, image.Point) {
		dims := dimensions[index]
		if dims.X == 0 || dims.Y == 0 {
			dims.X, dims.Y = 1, 1
		}
		rat := float64(dims.X) / float64(dims.Y)
		dims.X, dims.Y = r.realWidth, int(float64(r.realWidth)/rat)
		if float64(r.realHeight)/float64(dims.Y) < float64(r.realWidth)/float64(dims.X) {
			dims.X, dims.Y = int(float64(r.realHeight)*rat), r.realHeight
		}

		if !r.invalidateVAOs {
			return vaos[index], dims
		}
		if vbos[index] == 0 {
			return vaos[index], dims
		}

		r.invalidateVAOs = false
		gl.BindBuffer(gl.ARRAY_BUFFER, vbos[index]-1)

		d := points{}
		buf(&d, 0, 0, float32(dims.X), float32(dims.Y))
		gl.BufferData(gl.ARRAY_BUFFER, stride*vertices*fs, gl.Ptr(&d[0]), gl.DYNAMIC_DRAW)
		gl.BindBuffer(gl.ARRAY_BUFFER, 0)
		return vaos[index], dims
	}

	update := func() error {
		vao, dimension = getVAO(0)
		img, upd := r.get()
		if img == nil || !upd {
			return nil
		}
		newEntry(0, img.Bounds())

		if textures[0] != 0 {
			err = releaseTexture(textures[0] - 1)
			if err != nil {
				return err
			}
		}
		stex, err := imgTexture(img)

		tex = stex + 1
		vao, dimension = getVAO(0)

		if err != nil {
			return err
		}
		textures[0] = tex
		return nil
	}

	var lastDim image.Point
	frame := func() error {
		if err = update(); err != nil {
			return err
		}
		if tex == 0 {
			return nil
		}
		recenter := false
		if tex != lastTex {
			lastTex = tex
			gl.BindTexture(gl.TEXTURE_2D, uint32(tex-1))
			gl.BindVertexArray(vao - 1)
		}

		if r.proj != lastProjection {
			gl.UniformMatrix4fv(projectionUniform, 1, false, &r.proj[0])
			lastProjection = r.proj
			recenter = true
		}

		if dimension != lastDim {
			lastDim = dimension
			recenter = true
		}

		if recenter {
			tx := r.realWidth/2 - dimension.X/2
			ty := r.realHeight/2 - dimension.Y/2
			model = mgl32.Translate3D(float32(tx), float32(ty), 0)
			gl.UniformMatrix4fv(modelUniform, 1, false, &model[0])
		}

		gl.DrawElements(gl.TRIANGLES, 6, gl.UNSIGNED_INT, gl.PtrOffset(0))
		return nil
	}

	for !r.window.ShouldClose() {
		gl.Clear(gl.COLOR_BUFFER_BIT)
		if err = frame(); err != nil {
			return err
		}
		r.window.SwapBuffers()
		glfw.PollEvents()
	}

	return r.gErr
}
