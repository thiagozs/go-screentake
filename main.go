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

type App struct {
	// imagem atual exibida na janela (pode ser 1 monitor ou todos)
	bg    *ebiten.Image
	rawBG *image.RGBA // mesma imagem como image.RGBA para recorte

	// estado da seleção
	selecting   bool
	startX      int
	startY      int
	curX        int
	curY        int
	savedPath   string
	infoMessage string

	// multi-monitor
	displays []image.Rectangle // bounds absolutos dos monitores
	curDisp  int               // índice do monitor atual (no modo 1 monitor)
	modeAll  bool              // false = 1 monitor; true = todos
}

func main() {
	// lista de monitores
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		fmt.Println("Nenhum display ativo.")
		return
	}
	displays := make([]image.Rectangle, 0, n)
	for i := 0; i < n; i++ {
		displays = append(displays, screenshot.GetDisplayBounds(i))
	}

	// começa no primeiro (geralmente o primário)
	raw := captureDisplay(0)
	bg := ebiten.NewImageFromImage(raw)

	app := &App{
		bg:          bg,
		rawBG:       raw,
		infoMessage: "Clique e arraste para selecionar. Solte para salvar. Esc: sair | Q/E trocar monitor | A alterna 'todos'",
		displays:    displays,
		curDisp:     0,
		modeAll:     false,
	}

	w, h := raw.Bounds().Dx(), raw.Bounds().Dy()
	ebiten.SetWindowSize(min(w, 1600), min(h, 900))
	ebiten.SetWindowTitle("Snip - Seleção de área (1 monitor)")
	ebiten.SetWindowResizable(true)
	// se preferir fullscreen, habilite:
	// ebiten.SetFullscreen(true)

	if err := ebiten.RunGame(app); err != nil {
		panic(err)
	}
}

func (a *App) Update() error {
	// Sair com Esc
	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}

	// Alternar modo (A): 1 monitor <-> todos monitores
	if inpututil.IsKeyJustPressed(ebiten.KeyA) {
		a.modeAll = !a.modeAll
		a.selecting = false
		a.savedPath = ""
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
	}

	// Trocar monitor com Q/E (somente no modo 1 monitor)
	if !a.modeAll && inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		a.switchDisplay((a.curDisp - 1 + len(a.displays)) % len(a.displays))
	}
	if !a.modeAll && inpututil.IsKeyJustPressed(ebiten.KeyE) {
		a.switchDisplay((a.curDisp + 1) % len(a.displays))
	}

	// Posição atual do mouse
	x, y := ebiten.CursorPosition()

	// Início da seleção com botão esquerdo
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if !a.selecting {
			a.selecting = true
			a.startX, a.startY = x, y
			a.curX, a.curY = x, y
			a.savedPath = ""
		} else {
			a.curX, a.curY = x, y
		}
	} else {
		// Ao soltar, se estava selecionando, salva
		if a.selecting {
			a.selecting = false
			a.curX, a.curY = x, y
			a.saveSelection()
		}
	}
	return nil
}

func (a *App) Draw(screen *ebiten.Image) {
	// fundo: screenshot
	op := &ebiten.DrawImageOptions{}
	screen.DrawImage(a.bg, op)

	// desenha o retângulo de seleção (se estiver arrastando)
	if a.selecting {
		x0, y0, x1, y1 := normRect(a.startX, a.startY, a.curX, a.curY)
		// overlay semi-transparente por cima de tudo
		overlay := ebiten.NewImage(screen.Bounds().Dx(), screen.Bounds().Dy())
		overlay.Fill(color.NRGBA{R: 0, G: 0, B: 0, A: 100})
		screen.DrawImage(overlay, &ebiten.DrawImageOptions{})
		// “fura” o overlay com a área selecionada (desenha a bg de novo só naquela área)
		sub := a.bg.SubImage(image.Rect(x0, y0, x1, y1)).(*ebiten.Image)
		op2 := &ebiten.DrawImageOptions{}
		op2.GeoM.Translate(float64(x0), float64(y0))
		screen.DrawImage(sub, op2)

		// borda simples (linhas)
		drawRectBorder(screen, x0, y0, x1, y1)
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
	// Usa o tamanho do screenshot como canvas lógico
	return a.bg.Bounds().Dx(), a.bg.Bounds().Dy()
}

// --- utilidades de desenho ---

func drawRectBorder(dst *ebiten.Image, x0, y0, x1, y1 int) {
	w := x1 - x0
	h := y1 - y0
	th := 2 // espessura

	// cria uma imagem sólida para desenhar linhas
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

func normRect(x0, y0, x1, y1 int) (int, int, int, int) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	return x0, y0, x1, y1
}

// --- captura / salvamento / troca de display ---

// captura só 1 display e normaliza para (0,0)-(w,h)
func captureDisplay(index int) *image.RGBA {
	b := screenshot.GetDisplayBounds(index)
	img, err := screenshot.CaptureRect(b)
	if err != nil {
		fmt.Println("Erro ao capturar display:", index, err)
		// fallback: imagem vazia para não quebrar
		dst := image.NewRGBA(image.Rect(0, 0, 800, 600))
		return dst
	}
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Src)
	return dst
}

// junta todos os monitores numa única imagem normalizada a partir de (0,0)
func captureAllDisplays() *image.RGBA {
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		return nil
	}

	// Calcula retângulo total (monitores lado a lado, offsets podem ser negativos)
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

	// Captura cada display e compõe no destino corrigindo o offset
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

func (a *App) saveSelection() {
	x0, y0, x1, y1 := normRect(a.startX, a.startY, a.curX, a.curY)
	if x1-x0 < 2 || y1-y0 < 2 {
		a.infoMessage = "Seleção muito pequena. Tente novamente."
		return
	}

	// Recorta da imagem original
	rect := image.Rect(x0, y0, x1, y1)
	sub := a.rawBG.SubImage(rect)

	// Caminho de saída
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
	a.infoMessage = "Imagem salva!"
}

func (a *App) switchDisplay(index int) {
	if index < 0 || index >= len(a.displays) {
		return
	}
	a.curDisp = index
	a.selecting = false
	a.savedPath = ""
	a.infoMessage = "Monitor alterado. Clique e arraste para selecionar. Solte para salvar. Esc: sair | Q/E p/ trocar | A alterna 'todos'"

	raw := captureDisplay(index)
	a.rawBG = raw
	a.bg = ebiten.NewImageFromImage(raw)
}

func picturesDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Pictures")
	case "windows":
		return filepath.Join(home, "Pictures")
	default: // linux/bsd
		return filepath.Join(home, "Pictures")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
