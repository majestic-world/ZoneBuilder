# 03: Leitor de SkeletalMesh e MeshAnimation

**What to build:** o módulo `skeletal`, usado só pelo `zbmodel` (spec D6), lê de pacotes 132/40 do cliente uma SkeletalMesh (esqueleto, LOD 0 com a montagem das wedges do Lineage, até 4 influências por vértice, seções com material) e uma MeshAnimation (sequências com nome, taxa e frames, e as trilhas de cada osso). É um port das partes de `skeletal.rs`, `skin.rs`, `meshanim.rs` e `pose.rs` do UE2-Studio que esse caminho exercita; ler outras versões, gravar e PSK/PSA ficam de fora. O UE2-Studio é só leitura.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] Teste com cliente real (pulado sem `ZB_CLIENT`): `LineageMonsters15.death_knight_wizard_m00` é lido com LOD 0 consistente (todo índice aponta para uma wedge existente, todo peso de vértice soma 1, todo osso de influência existe) e 2 seções com nome de material
- [x] Teste com cliente real: `LineageMonsters15.death_knight_wizard_anim` tem a sequência `Wait`, com uma trilha por osso da mesh e frames > 0
- [x] Os números lidos batem com o que o UE2-Studio lê do mesmo pacote (contagem de ossos, wedges, faces e frames do `Wait`), registrado nas notas

## Comments

Integrado em `popular-zona`, a partir de `pz/03-leitor-de-skeletal-mesh` (`94d93ff`). Cliente Fafurion: 99 ossos, 2.741 vértices, 3.880 faces, 2 seções; Wait com 60 frames a 30 fps e 99 trilhas. Testes com `ZB_CLIENT`, build e vet passaram. O leitor usa o caminho de wedges clássicas quando presente, como o UE2-Studio; fallback Lineage quando ausente.

Comparação e API: `C:/Workspace/zone-builder-notes/popular-zona/03-leitor-de-skeletal-mesh.md` e `03-evidence/`.
