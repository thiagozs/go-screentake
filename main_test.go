package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// helper para comparar retângulos rapidamente
func rectEq(t *testing.T, got image.Rectangle, want image.Rectangle) {
	t.Helper()
	if got != want {
		t.Fatalf("rect mismatch: got=%v want=%v", got, want)
	}
}

func TestMin(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		if got := min(1, 2); got != 1 {
			t.Fatalf("min(1,2) = %d; want 1", got)
		}
		if got := min(5, -3); got != -3 {
			t.Fatalf("min(5,-3) = %d; want -3", got)
		}
	})
	t.Run("commutativity", func(t *testing.T) {
		cases := [][2]int{{0, 0}, {7, -2}, {-10, 3}, {1000, 999}}
		for _, c := range cases {
			a, b := c[0], c[1]
			if min(a, b) != min(b, a) {
				t.Fatalf("min not commutative for (%d,%d)", a, b)
			}
		}
	})
}

func TestNormRect(t *testing.T) {
	tests := []struct {
		name                           string
		x0, y0, x1, y1                 int
		wantX0, wantY0, wantX1, wantY1 int
	}{
		{"already_normal", 0, 0, 10, 10, 0, 0, 10, 10},
		{"swap_both", 10, 10, 0, 0, 0, 0, 10, 10},
		{"negatives", -5, 7, -10, 20, -10, 7, -5, 20},
		{"same_x", 3, 9, 3, 2, 3, 2, 3, 9},
		{"same_y", 2, 5, 7, 5, 2, 5, 7, 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			x0, y0, x1, y1 := normRect(tc.x0, tc.y0, tc.x1, tc.y1)
			if x0 != tc.wantX0 || y0 != tc.wantY0 || x1 != tc.wantX1 || y1 != tc.wantY1 {
				t.Fatalf("got (%d,%d,%d,%d); want (%d,%d,%d,%d)", x0, y0, x1, y1, tc.wantX0, tc.wantY0, tc.wantX1, tc.wantY1)
			}
		})
	}
}

func TestPicturesDir(t *testing.T) {
	dir := picturesDir()
	if filepath.Base(dir) != "Pictures" {
		t.Fatalf("picturesDir base = %q; want 'Pictures' (GOOS=%s)", filepath.Base(dir), runtime.GOOS)
	}
}

func TestClearSelection(t *testing.T) {
	app := &App{
		selecting:    true,
		hasSelection: true,
		startX:       10, startY: 20,
		curX: 30, curY: 40,
		selX0: 1, selY0: 2, selX1: 3, selY1: 4,
	}
	app.clearSelection()
	if app.selecting || app.hasSelection {
		t.Fatalf("selection flags not cleared: selecting=%v hasSelection=%v", app.selecting, app.hasSelection)
	}
	if app.startX != 0 || app.startY != 0 || app.curX != 0 || app.curY != 0 || app.selX0 != 0 || app.selY0 != 0 || app.selX1 != 0 || app.selY1 != 0 {
		t.Fatalf("selection coordinates not reset: got start(%d,%d) cur(%d,%d) rect(%d,%d,%d,%d)", app.startX, app.startY, app.curX, app.curY, app.selX0, app.selY0, app.selX1, app.selY1)
	}
}

func TestLayoutButtons(t *testing.T) {
	// Dado um canvas WxH, os botões ficam no canto inferior esquerdo com padding
	const (
		w       = 800
		h       = 600
		padding = 16
		btnW    = 120
		btnH    = 32
	)
	app := &App{}
	app.layoutButtons(w, h)

	wantSave := image.Rect(padding, h-padding-btnH, padding+btnW, h-padding)
	wantCancel := image.Rect(padding+btnW+8, h-padding-btnH, padding+btnW+8+btnW, h-padding)
	rectEq(t, app.saveBtn.Rect, wantSave)
	rectEq(t, app.cancelBtn.Rect, wantCancel)
}

func TestDoSaveWritesPNG(t *testing.T) {
	// HOME temporário para isolar saída em Pictures/
	tmp, err := os.MkdirTemp("", "gst-home-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	oldHome, had := os.LookupEnv("HOME")
	_ = os.Setenv("HOME", tmp)
	if had {
		t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv("HOME") })
	}

	// cria imagem base 50x50 com uma cor sólida
	raw := image.NewRGBA(image.Rect(0, 0, 50, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			raw.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	app := &App{rawBG: raw}
	// define uma seleção 10,10 até 20,20
	app.selX0, app.selY0, app.selX1, app.selY1 = 10, 10, 20, 20
	app.hasSelection = true

	app.doSave()
	if app.savedPath == "" {
		t.Fatalf("savedPath vazio; info=%q", app.infoMessage)
	}
	if app.hasSelection {
		t.Fatalf("seleção não foi limpa após salvar")
	}
	if _, err := os.Stat(app.savedPath); err != nil {
		t.Fatalf("arquivo salvo não encontrado: %s (%v)", app.savedPath, err)
	}
	// caminho deve estar dentro de HOME/Pictures
	wantPrefix := filepath.Join(tmp, "Pictures") + string(os.PathSeparator)
	if len(app.savedPath) <= len(wantPrefix) || app.savedPath[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("arquivo fora do diretório esperado: got=%s want_prefix=%s", app.savedPath, wantPrefix)
	}
	// sanity: nome com timestamp (prefixo snip-)
	if filepath.Base(app.savedPath) == "" || filepath.Ext(app.savedPath) != ".png" {
		t.Fatalf("nome de arquivo inesperado: %s", filepath.Base(app.savedPath))
	}
	// valida o conteúdo PNG: dimensões e um pixel amostral
	f, err := os.Open(app.savedPath)
	if err != nil {
		t.Fatalf("abrir arquivo salvo: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decodificar png: %v", err)
	}
	if img.Bounds().Dx() != 10 || img.Bounds().Dy() != 10 {
		t.Fatalf("png dimensões incorretas: got=%dx%d want=10x10", img.Bounds().Dx(), img.Bounds().Dy())
	}
	r, g, b, a := img.At(0, 0).RGBA()
	if r>>8 != 200 || g>>8 != 100 || b>>8 != 50 || a>>8 != 255 {
		t.Fatalf("pixel (0,0) inesperado: got=(%d,%d,%d,%d) want=(200,100,50,255)", r>>8, g>>8, b>>8, a>>8)
	}
	// small wait ensures filesystem flush (usual not required, but harmless)
	time.Sleep(10 * time.Millisecond)
}

func TestDoSaveFailsWhenPicturesIsAFile(t *testing.T) {
	// HOME temporário
	tmp, err := os.MkdirTemp("", "gst-home-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	oldHome, had := os.LookupEnv("HOME")
	_ = os.Setenv("HOME", tmp)
	if had {
		t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv("HOME") })
	}

	// cria um arquivo no caminho onde deveria ser o diretório Pictures
	picsPath := filepath.Join(tmp, "Pictures")
	if err := os.WriteFile(picsPath, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("criando arquivo Pictures: %v", err)
	}

	raw := image.NewRGBA(image.Rect(0, 0, 10, 10))
	app := &App{rawBG: raw}
	app.selX0, app.selY0, app.selX1, app.selY1 = 0, 0, 5, 5
	app.hasSelection = true
	app.doSave()
	if app.savedPath != "" {
		t.Fatalf("esperava falha; salvou em %s", app.savedPath)
	}
	if app.infoMessage == "" || !strings.HasPrefix(app.infoMessage, "Falha ao criar diretório") {
		t.Fatalf("mensagem inesperada: %q", app.infoMessage)
	}
}

func TestDoSaveFailsOnUnwritableDir(t *testing.T) {
	// HOME temporário
	tmp, err := os.MkdirTemp("", "gst-home-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	oldHome, had := os.LookupEnv("HOME")
	_ = os.Setenv("HOME", tmp)
	if had {
		t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv("HOME") })
	}

	picsDir := filepath.Join(tmp, "Pictures")
	if err := os.MkdirAll(picsDir, 0o555); err != nil { // sem permissão de escrita
		t.Fatalf("criando diretório Pictures: %v", err)
	}

	raw := image.NewRGBA(image.Rect(0, 0, 10, 10))
	app := &App{rawBG: raw}
	app.selX0, app.selY0, app.selX1, app.selY1 = 0, 0, 5, 5
	app.hasSelection = true
	app.doSave()
	if app.savedPath != "" {
		t.Fatalf("esperava falha; salvou em %s", app.savedPath)
	}
	if app.infoMessage == "" || !strings.HasPrefix(app.infoMessage, "Erro ao salvar arquivo") {
		t.Fatalf("mensagem inesperada: %q", app.infoMessage)
	}
}

func TestDoSaveWithoutSelection(t *testing.T) {
	// HOME temporário
	tmp, err := os.MkdirTemp("", "gst-home-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	oldHome, had := os.LookupEnv("HOME")
	_ = os.Setenv("HOME", tmp)
	if had {
		t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv("HOME") })
	}

	raw := image.NewRGBA(image.Rect(0, 0, 20, 20))
	app := &App{rawBG: raw, infoMessage: "keep"}
	// não há seleção
	app.hasSelection = false
	app.doSave()
	if app.savedPath != "" {
		t.Fatalf("salvou sem seleção: savedPath=%s", app.savedPath)
	}
	if app.infoMessage != "keep" {
		t.Fatalf("infoMessage alterada indevidamente: %q", app.infoMessage)
	}
	// diretório Pictures não deve existir
	dir := picturesDir()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("diretório de saída inesperadamente criado: %s", dir)
	}
}
