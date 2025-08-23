package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/kbinani/screenshot"
)

type Button struct {
	Rect  image.Rectangle
	Label string
	Hot   rune // atalho exibido no label, opcional (ex.: 'S' para salvar)
}

func (b Button) Draw(screen *ebiten.Image, hovered bool) {
	bg := ebiten.NewImage(b.Rect.Dx(), b.Rect.Dy())
	if hovered {
		bg.Fill(color.NRGBA{R: 60, G: 60, B: 60, A: 255})
	} else {
		bg.Fill(color.NRGBA{R: 40, G: 40, B: 40, A: 220})
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(b.Rect.Min.X), float64(b.Rect.Min.Y))
	screen.DrawImage(bg, op)

	// borda
	drawRectBorder(screen, b.Rect.Min.X, b.Rect.Min.Y, b.Rect.Max.X, b.Rect.Max.Y)

	// texto
	ebitenutil.DebugPrintAt(screen, b.Label, b.Rect.Min.X+10, b.Rect.Min.Y+8)
}

func (b Button) Contains(x, y int) bool {
	return image.Pt(x, y).In(b.Rect)
}

type App struct {
	bg    *ebiten.Image
	rawBG *image.RGBA

	// seleção
	selecting      bool // mouse arrastando
	hasSelection   bool // seleção finalizada (visível até salvar/cancelar)
	selX0, selY0   int
	selX1, selY1   int
	startX, startY int
	curX, curY     int

	// UI/estado
	saveBtn     Button
	cancelBtn   Button
	savedPath   string
	infoMessage string

	// multi-monitor
	displays []image.Rectangle
	curDisp  int
	modeAll  bool
}

// Valores de versão embutidos via -ldflags (ver Makefile)
var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		fmt.Println("Nenhum display ativo.")
		return
	}
	displays := make([]image.Rectangle, 0, n)
	for i := 0; i < n; i++ {
		displays = append(displays, screenshot.GetDisplayBounds(i))
	}

	raw := captureDisplay(0)
	bg := ebiten.NewImageFromImage(raw)

	app := &App{
		bg:          bg,
		rawBG:       raw,
		infoMessage: "Arraste para selecionar. Solte para ver opções. Enter=Salvar | Esc=Cancelar | Q/E trocar monitor | A 'todos'",
		displays:    displays,
		curDisp:     0,
		modeAll:     false,
	}
	app.layoutButtons(raw.Bounds().Dx(), raw.Bounds().Dy())

	w, h := raw.Bounds().Dx(), raw.Bounds().Dy()
	ebiten.SetWindowSize(min(w, 1600), min(h, 900))
	title := "Snip - Seleção de área (1 monitor)"
	if version != "" {
		ver := version
		if commit != "" {
			ver = fmt.Sprintf("%s (%s)", version, commit)
		}
		if date != "" {
			ver = fmt.Sprintf("%s - %s", ver, date)
		}
		title = fmt.Sprintf("%s | v%s", title, ver)
	}
	ebiten.SetWindowTitle(title)
	ebiten.SetWindowResizable(true)
	// ebiten.SetFullscreen(true)

	if err := ebiten.RunGame(app); err != nil {
		panic(err)
	}
}

func (a *App) Update() error {
	// Sair/cancelar com Esc
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if a.hasSelection || a.selecting {
			a.clearSelection()
			a.infoMessage = "Seleção cancelada."
		} else {
			return ebiten.Termination
		}
	}

	// Enter = salvar (se houver seleção pronta)
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && a.hasSelection {
		a.doSave()
	}

	// Alternar modo (A)
	if inpututil.IsKeyJustPressed(ebiten.KeyA) {
		a.modeAll = !a.modeAll
		a.clearSelection()
		if a.modeAll {
			raw := captureAllDisplays()
			a.rawBG = raw
			a.bg = ebiten.NewImageFromImage(raw)
			ebiten.SetWindowTitle("Snip - Seleção de área (todos monitores)")
		} else {
			raw := captureDisplay(a.curDisp)
			a.rawBG = raw
			a.bg = ebiten.NewImageFromImage(raw)
			ebiten.SetWindowTitle("Snip - Seleção de área (1 monitor)")
		}
		a.layoutButtons(a.rawBG.Bounds().Dx(), a.rawBG.Bounds().Dy())
	}

	// Trocar monitor (apenas modo 1 monitor)
	if !a.modeAll && inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		a.switchDisplay((a.curDisp - 1 + len(a.displays)) % len(a.displays))
	}
	if !a.modeAll && inpututil.IsKeyJustPressed(ebiten.KeyE) {
		a.switchDisplay((a.curDisp + 1) % len(a.displays))
	}

	// Mouse
	mx, my := ebiten.CursorPosition()

	// Início do arrasto
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if !a.selecting && !a.hasSelection {
			a.selecting = true
			a.startX, a.startY = mx, my
			a.curX, a.curY = mx, my
			a.savedPath = ""
		} else if a.selecting {
			a.curX, a.curY = mx, my
		}
	} else {
		// final do arrasto -> trava seleção (não salva)
		if a.selecting {
			a.selecting = false
			x0, y0, x1, y1 := normRect(a.startX, a.startY, a.curX, a.curY)
			if x1-x0 >= 2 && y1-y0 >= 2 {
				a.selX0, a.selY0, a.selX1, a.selY1 = x0, y0, x1, y1
				a.hasSelection = true
				a.infoMessage = "Seleção pronta. Use Salvar/Enter ou Cancelar/Esc."
			} else {
				a.infoMessage = "Seleção pequena. Tente novamente."
				a.clearSelection()
			}
		}
	}

	// Click nos botões (quando há seleção)
	if a.hasSelection && inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		if a.saveBtn.Contains(mx, my) {
			a.doSave()
		} else if a.cancelBtn.Contains(mx, my) {
			a.clearSelection()
			a.infoMessage = "Seleção cancelada."
		}
	}

	return nil
}

func (a *App) Draw(screen *ebiten.Image) {
	// fundo: screenshot
	op := &ebiten.DrawImageOptions{}
	screen.DrawImage(a.bg, op)

	// durante o arrasto: desenha overlay e borda da seleção
	if a.selecting {
		x0, y0, x1, y1 := normRect(a.startX, a.startY, a.curX, a.curY)
		drawOverlayWithHole(screen, a.bg, x0, y0, x1, y1)
		drawRectBorder(screen, x0, y0, x1, y1)
	}

	// após finalizar seleção (travada), desenha overlay/borda e os botões
	if a.hasSelection {
		drawOverlayWithHole(screen, a.bg, a.selX0, a.selY0, a.selX1, a.selY1)
		drawRectBorder(screen, a.selX0, a.selY0, a.selX1, a.selY1)

		mx, my := ebiten.CursorPosition()
		a.saveBtn.Draw(screen, a.saveBtn.Contains(mx, my))
		a.cancelBtn.Draw(screen, a.cancelBtn.Contains(mx, my))
	}

	// mensagens
	if a.infoMessage != "" {
		ebitenutil.DebugPrintAt(screen, a.infoMessage, 16, 16)
	}
	if a.savedPath != "" {
		ebitenutil.DebugPrintAt(screen, "Salvo em: "+a.savedPath, 16, 36)
	}

	// info de monitor / modo
	mode := "1 monitor"
	if a.modeAll {
		mode = "todos monitores"
	}
	dispInfo := fmt.Sprintf("Modo: %s | Monitor %d/%d", mode, a.curDisp+1, len(a.displays))
	ebitenutil.DebugPrintAt(screen, dispInfo, 16, 56)
}

func (a *App) Layout(outsideWidth, outsideHeight int) (int, int) {
	return a.bg.Bounds().Dx(), a.bg.Bounds().Dy()
}

// ---- helpers de desenho ----

func drawRectBorder(dst *ebiten.Image, x0, y0, x1, y1 int) {
	w := x1 - x0
	h := y1 - y0
	th := 2

	line := ebiten.NewImage(1, 1)
	line.Fill(color.NRGBA{R: 255, G: 255, B: 255, A: 220})

	// top
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(w), float64(th))
	op.GeoM.Translate(float64(x0), float64(y0))
	dst.DrawImage(line, op)
	// bottom
	op = &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(w), float64(th))
	op.GeoM.Translate(float64(x0), float64(y1-th))
	dst.DrawImage(line, op)
	// left
	op = &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(th), float64(h))
	op.GeoM.Translate(float64(x0), float64(y0))
	dst.DrawImage(line, op)
	// right
	op = &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(th), float64(h))
	op.GeoM.Translate(float64(x1-th), float64(y0))
	dst.DrawImage(line, op)
}

func drawOverlayWithHole(screen, bg *ebiten.Image, x0, y0, x1, y1 int) {
	overlay := ebiten.NewImage(screen.Bounds().Dx(), screen.Bounds().Dy())
	overlay.Fill(color.NRGBA{R: 0, G: 0, B: 0, A: 100})
	screen.DrawImage(overlay, &ebiten.DrawImageOptions{})

	sub := bg.SubImage(image.Rect(x0, y0, x1, y1)).(*ebiten.Image)
	op2 := &ebiten.DrawImageOptions{}
	op2.GeoM.Translate(float64(x0), float64(y0))
	screen.DrawImage(sub, op2)
}

// ---- captura / UI / salvar ----

func captureDisplay(index int) *image.RGBA {
	b := screenshot.GetDisplayBounds(index)
	img, err := screenshot.CaptureRect(b)
	if err != nil {
		fmt.Println("Erro ao capturar display:", index, err)
		dst := image.NewRGBA(image.Rect(0, 0, 800, 600))
		return dst
	}
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Src)
	return dst
}

func captureAllDisplays() *image.RGBA {
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		return nil
	}
	minX, minY := 1<<30, 1<<30
	maxX, maxY := -1<<30, -1<<30
	bounds := make([]image.Rectangle, 0, n)
	for i := 0; i < n; i++ {
		b := screenshot.GetDisplayBounds(i)
		bounds = append(bounds, b)
		if b.Min.X < minX {
			minX = b.Min.X
		}
		if b.Min.Y < minY {
			minY = b.Min.Y
		}
		if b.Max.X > maxX {
			maxX = b.Max.X
		}
		if b.Max.Y > maxY {
			maxY = b.Max.Y
		}
	}
	total := image.Rect(0, 0, maxX-minX, maxY-minY)
	dst := image.NewRGBA(total)
	for i := 0; i < n; i++ {
		b := bounds[i]
		img, err := screenshot.CaptureRect(b)
		if err != nil {
			fmt.Println("Erro ao capturar display:", i, err)
			continue
		}
		at := image.Pt(b.Min.X-minX, b.Min.Y-minY)
		r := image.Rectangle{Min: at, Max: at.Add(img.Bounds().Size())}
		draw.Draw(dst, r, img, image.Point{}, draw.Src)
	}
	return dst
}

func (a *App) switchDisplay(index int) {
	if index < 0 || index >= len(a.displays) {
		return
	}
	a.curDisp = index
	a.clearSelection()
	raw := captureDisplay(index)
	a.rawBG = raw
	a.bg = ebiten.NewImageFromImage(raw)
	a.layoutButtons(raw.Bounds().Dx(), raw.Bounds().Dy())
	a.infoMessage = "Monitor alterado. Arraste para selecionar. Enter=Salvar, Esc=Cancelar."
}

func (a *App) layoutButtons(w, h int) {
	// coloca os botões na parte inferior esquerda
	btnW, btnH := 120, 32
	padding := 16
	a.saveBtn = Button{
		Rect:  image.Rect(padding, h-padding-btnH, padding+btnW, h-padding),
		Label: "[Enter] Salvar",
		Hot:   'S',
	}
	a.cancelBtn = Button{
		Rect:  image.Rect(padding+btnW+8, h-padding-btnH, padding+btnW+8+btnW, h-padding),
		Label: "[Esc] Cancelar",
		Hot:   'C',
	}
}

func (a *App) doSave() {
	if !a.hasSelection {
		return
	}
	rect := image.Rect(a.selX0, a.selY0, a.selX1, a.selY1)
	sub := a.rawBG.SubImage(rect)

	dir := picturesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		a.infoMessage = "Falha ao criar diretório de saída: " + err.Error()
		return
	}
	name := "snip-" + time.Now().Format("20060102-150405") + ".png"
	path := filepath.Join(dir, name)

	var buf bytes.Buffer
	if err := png.Encode(&buf, sub); err != nil {
		a.infoMessage = "Erro ao codificar PNG: " + err.Error()
		return
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		a.infoMessage = "Erro ao salvar arquivo: " + err.Error()
		return
	}
	a.savedPath = path
	a.infoMessage = "Imagem salva! " + path
	a.clearSelection() // limpa após salvar
}

func (a *App) clearSelection() {
	a.selecting = false
	a.hasSelection = false
	a.startX, a.startY = 0, 0
	a.curX, a.curY = 0, 0
	a.selX0, a.selY0, a.selX1, a.selY1 = 0, 0, 0, 0
}

func picturesDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Pictures")
	case "windows":
		return filepath.Join(home, "Pictures")
	default:
		return filepath.Join(home, "Pictures")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func normRect(x0, y0, x1, y1 int) (int, int, int, int) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	return x0, y0, x1, y1
}
