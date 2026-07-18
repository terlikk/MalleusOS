# ============================================================
# MalleusOS — polecenia budowania
#
# Makefile to lista skrótów: wpisujesz `make build`, a make
# wykonuje przypisane polecenia. Wcięcia MUSZĄ być tabulatorami.
#
# CGO_ENABLED=0 wyłącza mostek Go→C, dzięki czemu binarka jest
# w pełni statyczna (zero zależności systemowych) i daje się
# krzyżowo kompilować na inne procesory jednym poleceniem.
# -trimpath i -ldflags "-s -w" odchudzają plik wynikowy.
# ============================================================

# ?= pozwala nadpisać wersję z zewnątrz: make build-all VERSION=v0.1.0
# (tak robi CI przy wydaniu). -X wpisuje ją w zmienną main.version.
VERSION ?= 0.1.0-dev
FLAGS    = -trimpath -ldflags "-s -w -X main.version=$(VERSION)"

# Zbuduj binarkę na bieżącą maszynę → bin/malleus
build:
	CGO_ENABLED=0 go build $(FLAGS) -o bin/malleus .

# Kompilacja krzyżowa: PC/serwery (amd64) + Raspberry Pi (arm64)
build-all:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(FLAGS) -o bin/malleus-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(FLAGS) -o bin/malleus-linux-arm64 .

# Statyczna analiza kodu — wyłapuje typowe błędy przed uruchomieniem
vet:
	go vet ./...

# Zbuduj i od razu uruchom lokalnie
run: build
	./bin/malleus

.PHONY: build build-all vet run web

# Zbuduj panel WWW (wynik: web/dist — wkompilowywany w binarkę).
# Pełna binarka z panelem: make web && make build
web:
	cd web && npm install && npm run build
