// Package model reads, writes, poses and CPU-skins UE2HUM01 bundles: a port of
// UE2-Studio's play_map/avatar (bundle.rs, avatar.rs) and animation-engine
// pose.rs. Coordinates are Unreal's (Z up); no client is read at runtime.
package model

import _ "embed"

// Human is UE2-Studio's assets/play-map/human.bin, copied byte for byte (see
// assets/README.md): the Human Fighter the game mode drives.
//
//go:embed assets/human.bin
var Human []byte
