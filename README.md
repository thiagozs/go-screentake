# go-screentake

![CI](https://github.com/thiagozs/go-screentake/actions/workflows/ci.yml/badge.svg)

Ferramenta GUI simples para selecionar uma área da tela e salvar como PNG, usando Ebiten.

## Requisitos

- Go 1.23+
- Linux/X11: bibliotecas de sistema (X11/OpenGL/ALSA) para Ebiten/GLFW
  - Instalação automática (Debian/Ubuntu, Fedora, Arch):

  ```bash
  make deps-linux
  ```

## Build e execução

- Compilar (gera `bin/go-screentake`):

  ```bash
  make build
  ```

- Executar sem compilar binário:

  ```bash
  make run
  ```

## Makefile - alvos úteis

- Qualidade e manutenção:
  - `make fmt` - formata
  - `make vet` - análise estática
  - `make test` - testes
  - `make tidy` - `go mod tidy`
  - `make lint` - roda golangci-lint (use `make lint-install` para instalar)

- Versão e metadados:
  - `make version` - exibe VERSION/COMMIT/DATE
  - Embutir versão custom: `make build VERSION=1.2.3`

- Cross-compile rápido:
  - `make build-linux` / `make build-linux-arm64`
  - `make build-windows` / `make build-windows-arm64`
  - `make build-darwin` / `make build-darwin-arm64`
  - (Experimental) `make build-darwin-osxcross` / `make build-darwin-arm64-osxcross`

- Release completa (linux/windows/darwin amd64/arm64) + checksums em `dist/` (pacotes incluem README e LICENSE quando disponíveis):

  ```bash
  make release
  ```

- Limpar binários:

  ```bash
  make clean
  ```

## Observações

- CGO precisa estar habilitado (padrão no Makefile) para GLFW/Ebiten.
- Cross-compile com CGO pode exigir toolchains específicos por plataforma (p.ex. MinGW para Windows).
- O título da janela inclui versão/commit/data quando embutidos via `-ldflags`.
- A versão atual do projeto é mantida no arquivo `VERSION` e atualizada pelo alvo `make tag`.

## Cross-compile para macOS com osxcross (opcional)

Para compilar para macOS a partir do Linux com CGO:

1. Instale dependências do host (ex.: Ubuntu): `clang`, `llvm`, `cmake`, `make`, `pkg-config`, `git`.
2. Instale e prepare o osxcross (SDK da Apple necessário). Siga a documentação do projeto osxcross para obter o SDK legalmente.
3. Exporte o caminho do osxcross e rode os alvos:

  ```bash
  export OSXCROSS_PATH=/opt/osxcross
  # amd64
  make build-darwin-osxcross
  # arm64 (Apple Silicon)
  make build-darwin-arm64-osxcross
  ```

Observação: cross-compile com CGO pode variar conforme a versão do SDK/Clang. Para releases oficiais, prefira o workflow do GitHub Actions (runner macOS).

## Atalhos no app

- Arraste para selecionar
- Enter: salvar
- Esc: cancelar
- Q/E: trocar monitor (modo 1 monitor)
- A: alterna captura de todos os monitores

## Autor

2025, Desenvolvido por Thiago Zilli Sarmento :heart:
