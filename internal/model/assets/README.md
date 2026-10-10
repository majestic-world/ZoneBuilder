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
