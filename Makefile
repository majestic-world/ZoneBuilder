APP := zonebuilder
BIN := bin/$(APP).exe

# scripts/build.ps1 compiles without cgo and copies ANGLE's libEGL.dll and
# libGLESv2.dll next to the executable.
POWERSHELL ?= pwsh
# 64-bit ANGLE folder; override with `make build ZB_ANGLE_DIR=...` or the
# ZB_ANGLE_DIR environment variable.
ZB_ANGLE_DIR ?= C:\Program Files (x86)\Steam\bin\cef\cef.win64
# Flags for the app, e.g. `make run ARGS="-project giran.zbproj"`.
ARGS ?=

.PHONY: build run

build:
	$(POWERSHELL) -NoProfile -ExecutionPolicy Bypass -File scripts/build.ps1 -AngleDir "$(ZB_ANGLE_DIR)"

run: build
	./$(BIN) $(ARGS)
