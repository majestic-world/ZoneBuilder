BIN := bin/Zone Builder.exe

# scripts/build.ps1 compiles without cgo and copies ANGLE's libEGL.dll and
# libGLESv2.dll from third_party/angle next to the executable.
POWERSHELL ?= pwsh
# Flags for the app, e.g. `make run ARGS="-project giran.zbproj"`.
ARGS ?=

.PHONY: build run dist

build:
	$(POWERSHELL) -NoProfile -ExecutionPolicy Bypass -File scripts/build.ps1

run: build
	"./$(BIN)" $(ARGS)

# Zips bin/ into "dist/Zone Builder By Mk v<APP_VERSION>.zip".
dist: build
	$(POWERSHELL) -NoProfile -ExecutionPolicy Bypass -File scripts/dist.ps1
