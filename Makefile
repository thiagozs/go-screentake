# Makefile para build/execução do app Go
# Uso: make [alvo]

# Configurações
APP            := gst
BIN_DIR        := bin
DIST_DIR       := dist
GO             ?= go
GOFLAGS        ?=
LDFLAGS        ?= -s -w

# Build metadata (pode ser sobrescrito: make build VERSION=1.2.3)
# Preferir arquivo VERSION, senão usar git describe; por fim, 'dev'
ifneq ("$(wildcard VERSION)","")
VERSION        := $(shell cat VERSION)
else
VERSION        ?= $(shell git describe --tags --always 2>/dev/null || echo dev)
endif
COMMIT         ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE           ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
# Data curta para changelog
DATE_SHORT     := $(shell date +%Y-%m-%d)
# Variáveis injetadas via -ldflags
LDVARS         := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

# Plataforma padrão (pega do ambiente do Go)
GOOS_DEFAULT   := $(shell $(GO) env GOOS)
GOARCH_DEFAULT := $(shell $(GO) env GOARCH)

# Alvo padrão
.DEFAULT_GOAL := build

.PHONY: help
help:
	@echo "Alvos disponíveis:"
	@echo "  build          - Compila para a plataforma atual em $(BIN_DIR)/$(APP)"
	@echo "  run            - Executa com go run (sem gerar binário)"
	@echo "  install        - Instala o módulo atual no GOPATH/bin"
	@echo "  test           - Roda testes (se existirem)"
	@echo "  tidy           - go mod tidy"
	@echo "  fmt            - go fmt ./..."
	@echo "  vet            - go vet ./..."
	@echo "  clean          - Remove diretório $(BIN_DIR)"
	@echo "  deps-linux     - Instala dependências do sistema (X11/OpenGL) em distros comuns"
	@echo "  version        - Mostra versão/commit/data que serão embutidos"
	@echo "  lint           - Roda golangci-lint (se instalado)"
	@echo "  lint-install   - Instala golangci-lint (Linux)"
	@echo "  build-linux    - Compila binário Linux amd64"
	@echo "  build-windows  - Compila binário Windows amd64 (.exe)"
	@echo "  build-darwin   - Compila binário macOS amd64"
	@echo "  build-linux-arm64  - Compila binário Linux arm64"
	@echo "  build-darwin-arm64 - Compila binário macOS arm64"
	@echo "  build-windows-arm64- Compila binário Windows arm64 (.exe)"
	@echo "  build-darwin-osxcross      - Cross-compile macOS amd64 (requer osxcross: CC=o64-clang)"
	@echo "  build-darwin-arm64-osxcross- Cross-compile macOS arm64 (requer osxcross: CC=oa64-clang)"
	@echo "  release        - Cross-compile (linux/windows/darwin amd64/arm64) + checksums em $(DIST_DIR)"
	@echo "  changelog-release VERSION=x.y.z - Cria seção do changelog para a versão"
	@echo "  tag VERSION=x.y.z - Atualiza CHANGELOG, cria tag e faz push"

# Garante diretório bin
$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

$(DIST_DIR):
	@mkdir -p $(DIST_DIR)

.PHONY: build
build: $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(LDVARS)" -o $(BIN_DIR)/$(APP) .
	@echo "Binário gerado: $(BIN_DIR)/$(APP) (GOOS=$(GOOS_DEFAULT) GOARCH=$(GOARCH_DEFAULT))"

.PHONY: run
run:
	$(GO) run $(GOFLAGS) .

.PHONY: install
install:
	$(GO) install $(GOFLAGS) ./...

.PHONY: test
test:
	EBITEN_HEADLESS=1 $(GO) test $(GOFLAGS) ./...

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

# Dependências do sistema no Linux (X11/OpenGL/GLFW) para CGO
.PHONY: deps-linux
deps-linux:
	@set -e; \
	if command -v apt >/dev/null 2>&1; then \
		echo "Detectado apt (Debian/Ubuntu). Instalando pacotes..."; \
		sudo apt update; \
		sudo apt install -y build-essential xorg-dev libgl1-mesa-dev libasound2-dev libxi-dev libxcursor-dev libxrandr-dev libxinerama-dev; \
	elif command -v dnf >/dev/null 2>&1; then \
		echo "Detectado dnf (Fedora). Instalando pacotes..."; \
		sudo dnf install -y @development-tools libX11-devel libXcursor-devel libXrandr-devel libXinerama-devel mesa-libGL-devel alsa-lib-devel; \
	elif command -v pacman >/dev/null 2>&1; then \
		echo "Detectado pacman (Arch). Instalando pacotes..."; \
		sudo pacman -S --needed --noconfirm base-devel libx11 libxcursor libxrandr libxinerama mesa alsa-lib; \
	else \
		echo "Gerenciador de pacotes não identificado. Instale manualmente as libs X11/OpenGL/ALSA"; \
		exit 1; \
	fi

# Cross-compilações comuns
.PHONY: build-linux
build-linux: $(BIN_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(LDVARS)" -o $(BIN_DIR)/$(APP)-linux-amd64 .

.PHONY: build-windows
build-windows: $(BIN_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(LDVARS)" -o $(BIN_DIR)/$(APP)-windows-amd64.exe .

.PHONY: build-darwin
build-darwin: $(BIN_DIR)
	@set -e; \
	if [ "$(shell uname -s)" = "Darwin" ]; then \
	  GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(LDVARS)" -o $(BIN_DIR)/$(APP)-darwin-amd64 .; \
	else \
	  echo "build-darwin: compile em um host macOS ou use o workflow de release (macOS runner)."; \
	  echo "Dica avançada: configure osxcross (o64-clang) para cross-compile com CGO."; \
	  exit 1; \
	fi

# ARM64 builds
.PHONY: build-linux-arm64
build-linux-arm64: $(BIN_DIR)
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(LDVARS)" -o $(BIN_DIR)/$(APP)-linux-arm64 .

.PHONY: build-darwin-arm64
build-darwin-arm64: $(BIN_DIR)
	@set -e; \
	if [ "$(shell uname -s)" = "Darwin" ]; then \
	  GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(LDVARS)" -o $(BIN_DIR)/$(APP)-darwin-arm64 .; \
	else \
	  echo "build-darwin-arm64: compile em um host macOS (Apple Silicon) ou use o workflow de release (macOS runner)."; \
	  echo "Dica avançada: configure osxcross (oa64-clang) para cross-compile com CGO."; \
	  exit 1; \
	fi

# Cross-compile via osxcross (experimental)
.PHONY: build-darwin-osxcross
build-darwin-osxcross: $(BIN_DIR)
	@set -e; \
	: $${OSXCROSS_PATH:?Defina OSXCROSS_PATH apontando para o osxcross}; \
	: $${MACOSX_DEPLOYMENT_TARGET:=10.13}; export MACOSX_DEPLOYMENT_TARGET; \
	export PATH="$$OSXCROSS_PATH/target/bin:$$PATH"; \
	CC=o64-clang CXX=o64-clang++ GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(LDVARS)" -o $(BIN_DIR)/$(APP)-darwin-amd64 .

.PHONY: build-darwin-arm64-osxcross
build-darwin-arm64-osxcross: $(BIN_DIR)
	@set -e; \
	: $${OSXCROSS_PATH:?Defina OSXCROSS_PATH apontando para o osxcross}; \
	: $${MACOSX_DEPLOYMENT_TARGET:=11.0}; export MACOSX_DEPLOYMENT_TARGET; \
	export PATH="$$OSXCROSS_PATH/target/bin:$$PATH"; \
	CC=oa64-clang CXX=oa64-clang++ GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(LDVARS)" -o $(BIN_DIR)/$(APP)-darwin-arm64 .

.PHONY: build-windows-arm64
build-windows-arm64: $(BIN_DIR)
	GOOS=windows GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(LDVARS)" -o $(BIN_DIR)/$(APP)-windows-arm64.exe .

# Version info
.PHONY: version
version:
	@echo "VERSION=$(VERSION)"
	@echo "COMMIT=$(COMMIT)"
	@echo "DATE=$(DATE)"

# Lint
.PHONY: lint
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint não encontrado. Use: make lint-install"; exit 1; }
	golangci-lint run

.PHONY: lint-install
lint-install:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$HOME/.local/bin v1.61.0
	@echo "Adicione $$HOME/.local/bin ao PATH se necessário."

# Verifica repositório git limpo
.PHONY: verify-clean
verify-clean:
	@command -v git >/dev/null 2>&1 || { echo "Git não encontrado"; exit 1; }
	@git rev-parse --git-dir >/dev/null 2>&1 || { echo "Este diretório não é um repositório git"; exit 1; }
	@test -z "$(shell git status --porcelain)" || { echo "Workspace com mudanças não commitadas. Commit ou stash antes."; exit 1; }

# Cria seção no CHANGELOG para uma versão (move itens de Unreleased para a nova seção)
.PHONY: changelog-release
changelog-release:
	@set -e; \
	: $${VERSION:?Informe VERSION=x.y.z}; \
	[ -f CHANGELOG.md ] || { echo "CHANGELOG.md não encontrado"; exit 1; }; \
	grep -q '^## \[Unreleased\]' CHANGELOG.md || { echo "Seção [Unreleased] não encontrada"; exit 1; }; \
	if grep -q "^## \[$${VERSION}\]" CHANGELOG.md; then echo "Versão $$VERSION já existe no CHANGELOG."; exit 0; fi; \
	tmpdir=$$(mktemp -d); \
	head="$$tmpdir/head.md"; content="$$tmpdir/content.md"; tailf="$$tmpdir/tail.md"; \
	sed -n '1,/^## \[Unreleased\]/p' CHANGELOG.md | sed '$$d' > "$$head"; \
	if sed -n '/^## \[Unreleased\]/,/^## \[/p' CHANGELOG.md | grep -q '^## \['; then \
	  sed -n '/^## \[Unreleased\]/,/^## \[/p' CHANGELOG.md | sed '1d;$$d' > "$$content"; \
	  sed -n '/^## \[Unreleased\]/,/^## \[/d; p' CHANGELOG.md > "$$tailf"; \
	else \
	  sed -n '/^## \[Unreleased\]/,$$p' CHANGELOG.md | sed '1d' > "$$content"; \
	  : > "$$tailf"; \
	fi; \
	{ \
	  cat "$$head"; echo; \
	  echo '## [Unreleased]'; echo; \
	  echo '- Em progresso'; echo; \
	  echo "## [$$VERSION] - $(DATE_SHORT)"; echo; \
	  cat "$$content"; \
	  cat "$$tailf"; \
	} > CHANGELOG.md.new; \
	mv CHANGELOG.md.new CHANGELOG.md; \
	rm -rf "$$tmpdir"; \
	echo "CHANGELOG atualizado para $$VERSION"

# Cria tag git e publica (usa changelog-release e commit automático)
.PHONY: tag
tag: verify-clean changelog-release
	@set -e; \
	: $${VERSION:?Informe VERSION=x.y.z}; \
	echo "$$VERSION" > VERSION; \
	git add CHANGELOG.md || true; \
	git add VERSION || true; \
	git commit -m "chore(release): v$$VERSION" || true; \
	git tag -a v$$VERSION -m "Release v$$VERSION"; \
	git push origin HEAD; \
	git push origin v$$VERSION

# Release: cross-compile + checksums
.PHONY: release
release: $(DIST_DIR)
	@set -e; \
	# Matrizes de build
	for target in \
	  linux amd64 '' \
	  linux arm64 '' \
	  windows amd64 .exe \
	  windows arm64 .exe \
	  darwin amd64 '' \
	  darwin arm64 '' \
	; do \
	  set -- $$target; GOOS=$$1; GOARCH=$$2; EXT=$$3; \
	  OUT="$(DIST_DIR)/$(APP)-$${GOOS}-$${GOARCH}$${EXT}"; \
	  echo "Building $$OUT"; \
	  GOOS=$$GOOS GOARCH=$$GOARCH $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(LDVARS)" -o "$$OUT" .; \
	  # Empacotar
	  PKG_BASE="$(APP)_$(VERSION)_$${GOOS}_$${GOARCH}"; \
	  STAGE_DIR="$(DIST_DIR)/$${PKG_BASE}"; \
	  rm -rf "$$STAGE_DIR"; mkdir -p "$$STAGE_DIR"; \
		if [ "$$GOOS" = "windows" ]; then \
			cp "$$OUT" "$$STAGE_DIR/$(APP).exe"; \
			[ -f README.md ] && cp README.md "$$STAGE_DIR/" || true; \
			[ -f CHANGELOG.md ] && cp CHANGELOG.md "$$STAGE_DIR/" || true; \
			for f in LICENSE LICENSE.md LICENSE.txt; do [ -f "$$f" ] && cp "$$f" "$$STAGE_DIR/"; done; \
	    if command -v zip >/dev/null 2>&1; then \
	      (cd "$(DIST_DIR)" && zip -rq "$${PKG_BASE}.zip" "$${PKG_BASE}"); \
	    elif command -v 7z >/dev/null 2>&1; then \
	      (cd "$(DIST_DIR)" && 7z a -tzip -bd -y "$${PKG_BASE}.zip" "$${PKG_BASE}" >/dev/null); \
	    else \
	      echo "zip/7z não encontrados, pacote Windows não gerado para $$PKG_BASE"; \
	    fi; \
	  else \
			cp "$$OUT" "$$STAGE_DIR/$(APP)"; \
			[ -f README.md ] && cp README.md "$$STAGE_DIR/" || true; \
			[ -f CHANGELOG.md ] && cp CHANGELOG.md "$$STAGE_DIR/" || true; \
			for f in LICENSE LICENSE.md LICENSE.txt; do [ -f "$$f" ] && cp "$$f" "$$STAGE_DIR/"; done; \
	    (cd "$(DIST_DIR)" && tar -czf "$${PKG_BASE}.tar.gz" "$${PKG_BASE}"); \
	  fi; \
	  rm -rf "$$STAGE_DIR"; \
	done; \
	# Checksums apenas dos pacotes gerados
	if command -v sha256sum >/dev/null 2>&1; then \
	  find "$(DIST_DIR)" -maxdepth 1 -type f \( -name '*.tar.gz' -o -name '*.zip' \) -exec sha256sum {} + > "$(DIST_DIR)/SHA256SUMS.txt"; \
	elif command -v shasum >/dev/null 2>&1; then \
	  find "$(DIST_DIR)" -maxdepth 1 -type f \( -name '*.tar.gz' -o -name '*.zip' \) -exec shasum -a 256 {} + > "$(DIST_DIR)/SHA256SUMS.txt"; \
	else \
	  echo "Nem sha256sum nem shasum encontrados; pulei checksums."; \
	fi

# Release de um único GOOS/GOARCH (para uso em CI por runner)
.PHONY: release-one
release-one: $(DIST_DIR)
	@set -e; \
	: $${GOOS:?GOOS não definido}; \
	: $${GOARCH:?GOARCH não definido}; \
	EXT=""; [ "$$GOOS" = "windows" ] && EXT=".exe" || true; \
	OUT="$(DIST_DIR)/$(APP)-$${GOOS}-$${GOARCH}$$EXT"; \
	echo "Building $$OUT"; \
	GOOS=$$GOOS GOARCH=$$GOARCH $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(LDVARS)" -o "$$OUT" .; \
	PKG_BASE="$(APP)_$(VERSION)_$${GOOS}_$${GOARCH}"; \
	STAGE_DIR="$(DIST_DIR)/$${PKG_BASE}"; \
	rm -rf "$$STAGE_DIR"; mkdir -p "$$STAGE_DIR"; \
	if [ "$$GOOS" = "windows" ]; then \
	  cp "$$OUT" "$$STAGE_DIR/$(APP).exe"; \
	  [ -f README.md ] && cp README.md "$$STAGE_DIR/" || true; \
	  for f in LICENSE LICENSE.md LICENSE.txt; do [ -f "$$f" ] && cp "$$f" "$$STAGE_DIR/"; done; \
	  if command -v zip >/dev/null 2>&1; then \
	    (cd "$(DIST_DIR)" && zip -rq "$${PKG_BASE}.zip" "$${PKG_BASE}"); \
	  elif command -v 7z >/dev/null 2>&1; then \
	    (cd "$(DIST_DIR)" && 7z a -tzip -bd -y "$${PKG_BASE}.zip" "$${PKG_BASE}" >/dev/null); \
	  else \
	    echo "zip/7z não encontrados, pacote Windows não gerado para $$PKG_BASE"; \
	  fi; \
	else \
	  cp "$$OUT" "$$STAGE_DIR/$(APP)"; \
	  [ -f README.md ] && cp README.md "$$STAGE_DIR/" || true; \
	  for f in LICENSE LICENSE.md LICENSE.txt; do [ -f "$$f" ] && cp "$$f" "$$STAGE_DIR/"; done; \
	  (cd "$(DIST_DIR)" && tar -czf "$${PKG_BASE}.tar.gz" "$${PKG_BASE}"); \
	fi; \
	rm -rf "$$STAGE_DIR"
