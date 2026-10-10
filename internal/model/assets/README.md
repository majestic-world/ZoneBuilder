# Embedded models

Embedded with `//go:embed` in `internal/model/embed.go` and decoded from memory with
`model.Decode`. Nothing here is read from a Lineage II client at runtime.

## `human.bin`

The Human Fighter wearing Dynasty Light that UE2-Studio's Play Map drives, in the
`UE2HUM01` format. Copied from UE2-Studio without changing a byte:

| | |
|---|---|
| Source | `C:/Workspace/UE2-Studio/assets/play-map/human.bin` |
| Source commit | `dd0d81bffa55a4636dade9a613a5e06c3d804183` (2026-10-01), last commit touching the file |
| Size | 1517904 bytes |
| SHA-256 | `2cde015f522973a95b3ad3b20b72e08ba6e0db07df1bfbb4e25e6c68a95d7423` |

Contents (see UE2-Studio's `assets/play-map/README.md`): 8 parts (Dynasty Leather Armor
upper and lower body, gloves, boots, face, 2 hair meshes and the right-shoulder accessory)
from the InterludeClassicRetail client's `Animations/Fighter.ukx`, skins from
`SysTextures/MFighter.utx` and `SysTextures/LineageFaceTex.utx`, and 5 clips of
`MFighter_anim` with root motion already removed: 0 idle (`wait_Hand_MFighter`),
1 run (`run_Hand_MFighter`), 2 falling (`Falling_MFighter`), 3 standing jump and
4 running jump (`JumpClips`). The placement scales the body to 80 units with the feet on
Z = 0 in idle frame 0.

Do not regenerate it here. UE2-Studio owns the extractor (the test-only
`Assets::load_client` in `src/play_map/avatar.rs`, run with
`cargo test --profile testing --bin UE2Studio client_human_bundle_preserves -- --ignored`
and `MAP_EDITOR_AVATAR_CLIENT` / `MAP_EDITOR_AVATAR_BUNDLE_OUT`); to update, copy its
`assets/play-map/human.bin` again and record the new commit and hash above. Check the copy:

```sh
sha256sum internal/model/assets/human.bin
```

`go test ./internal/model` proves `Encode(Decode(human.bin))` gives the same bytes.

## `monster.bin`

The preview monster, `death_knight_wizard_m00`, drawn at every spawn point whatever the
NPC id, in the `UE2HUM01` format. Extracted by this repo's `cmd/zbmodel` straight from the
Lineage II client; UE2-Studio is not involved.

| | |
|---|---|
| Client | Lineage II - Tale Of Aden - Fafurion |
| Mesh | `LineageMonsters15.death_knight_wizard_m00` (`Animations/LineageMonsters15.ukx`, SHA-256 `a2d612edb391bf9e3e08f9e2bc1981e6f4fa8a67d56b1f5b32b17824918ec8bc`) |
| Animation | `LineageMonsters15.death_knight_wizard_anim`, clip `Wait` (60 frames at 30 fps) |
| Skins | `LineageMonstersTex9.death_knight_wizard.death_knight_wizard_t00` and `_t01` (`SysTextures/LineageMonstersTex9.utx`, SHA-256 `047b469e55a21f27640c7c916041eb451d009ea34a0fc58a34dcabf72db17fe4`) |
| Scale | native (drawscale 1) |
| Size | 902552 bytes |
| SHA-256 | `41318fdb12f56ebbae036a1934b2ce6ee2e77e4ebdb501b2b56c720afbca287f` |

Contents: 1 part (2741 vertices, 99 bones, 2 sections whose 512×512 skins are the
`_t00_ori` and `_t01_ori` textures the FinalBlend → Shader → Combiner graphs lead to),
clip 0 `Wait` with track 0's X and Y pinned to its first key (root motion removed), and
clips 1 and 2 repeating it, since `Decode` needs 3 clips; `JumpClips` are 0. The
placement keeps drawscale 1 and puts the feet on Z = 0 in Wait frame 0.

`zbmodel` printed `raio 8.74579` and `altura 66.2339`, which are `MonsterRadius` and
`MonsterHeight` in `embed.go`. Regenerate from the repo root (Windows-style client path):

```sh
go run ./cmd/zbmodel -client "$ZB_CLIENT" \
  -mesh LineageMonsters15.death_knight_wizard_m00 \
  -anim LineageMonsters15.death_knight_wizard_anim \
  -skins LineageMonstersTex9.death_knight_wizard.death_knight_wizard_t00,LineageMonstersTex9.death_knight_wizard.death_knight_wizard_t01 \
  -clips Wait \
  -out internal/model/assets/monster.bin
```

Then update the constants from the printed radius and height, and the size and hash above.
With `ZB_CLIENT` set, `go test ./cmd/zbmodel` proves the command still gives these bytes and
these constants.
