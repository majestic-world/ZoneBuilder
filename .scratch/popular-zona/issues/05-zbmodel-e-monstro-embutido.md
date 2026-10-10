# 05: `zbmodel` e monstro embutido

**What to build:** a CLI offline `zbmodel` extrai o monstro de prévia direto do cliente (spec D6): flags de cliente, mesh, animação, skins, clips, drawscale (padrão 1) e saída. Lê a mesh e a animação pelo `skeletal`, resolve as skins pelo grafo de material de `internal/unreal`, decodifica para RGBA e grava como PNG, remove o root motion como o `Assets::load_client` do UE2-Studio, põe os pés em Z = 0 na pose do primeiro clip, grava no formato `UE2HUM01` pelo `model.Encode` e imprime o raio (mediana da distância horizontal dos vértices, a regra do `npcgrp.rs`) e a altura da pose. O `monster.bin` do `death_knight_wizard_m00` (mesh `LineageMonsters15.death_knight_wizard_m00`, animação `LineageMonsters15.death_knight_wizard_anim`, skins `LineageMonstersTex9.death_knight_wizard.death_knight_wizard_t00` e `_t01`, clip `Wait`, escala nativa) fica embutido ao lado do humano, com o raio e a altura medidos como constantes e o comando de regeneração no README de procedência.

**Blocked by:** 03, 04

**Status:** resolved

- [x] Teste com cliente real (pulado sem `ZB_CLIENT`): o `zbmodel` lê o `death_knight_wizard_m00`, e a pose `Wait` tem as 2 seções com textura (nenhuma rosa ou vazia) e o menor Z em 0
- [x] O `monster.bin` embutido decodifica com `model.Decode`, e regerá-lo com o comando do README dá os mesmos bytes
- [x] O raio e a altura impressos viram as constantes do monstro de prévia
- [x] Render da pose `Wait` (PNG de conferência nas notas) comparado com a captura do UE2-Studio: mesh, cajado e as 2 skins; divergência registrada

## Comments

Integrado em `popular-zona`, a partir de `pz/05-zbmodel-e-monstro-embutido` (`c785be4`). A CLI regenerou 902.552 bytes idênticos ao embed; raio 8,74579 e altura 66,2339. Testes das 2 skins e pés no chão passaram com `ZB_CLIENT`. Render offline comparado com captura do UE2-Studio: mesh, cajado na mão e skins coerentes; diferenças de câmera/luz/apresentação registradas. Build e vet passaram.

Comando e evidências: `C:/Workspace/zone-builder-notes/popular-zona/05-zbmodel-e-monstro-embutido.md` e `05-evidence/`; procedência em `internal/model/assets/README.md`.
