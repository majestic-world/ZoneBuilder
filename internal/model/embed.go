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

// Monster is the preview monster drawn at every spawn point whatever its NPC
// id: LineageMonsters15.death_knight_wizard_m00 at native scale, extracted
// by cmd/zbmodel (see assets/README.md). Clip 0 is Wait; clips 1 and 2
// repeat it and JumpClips are 0, since Decode needs 3 clips.
//
//go:embed assets/monster.bin
var Monster []byte

// The preview monster's measures, printed by the zbmodel command that
// generated Monster.
const (
	// MonsterRadius is the collision radius: npcgrp.rs's lower median of the
	// bind-pose vertices' horizontal distance from the actor axis.
	MonsterRadius float32 = 8.74579
	// MonsterHeight is the Z extent of Wait frame 0, feet to top.
	MonsterHeight float32 = 66.2339
)
