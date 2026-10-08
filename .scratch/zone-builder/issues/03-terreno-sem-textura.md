# 03: Terreno sem textura no viewport

**What to build:** o usuário aponta o app para a pasta do cliente, escolhe um tile `X_Y` e vê o terreno daquele mapa no viewport, sem textura, voando com a câmera livre do UE2-Studio. Isso cobre:
- as propriedades com tag (com o preâmbulo `RF_HAS_STACK`);
- o `TerrainInfo`;
- o heightmap G16;
- `TerrainScale`, `QuadVisibilityBitmap`, `EdgeTurnBitmap` e o fallback por `MapX/MapY`.

Este ticket abre o seam Cena (`Load`). A cena guarda coordenadas absolutas e expõe uma origem de rebase para o renderizador.

**Blocked by:** 01 (Spike de renderização Gio + ANGLE), 02 (Abrir pacotes e listar exports)

**Status:** resolved

- [x] Escolher a pasta do cliente e o tile Giran abre o terreno no lugar certo, com os quads invisíveis faltando onde o UE2-Studio também não desenha
- [x] O terreno ocupa a faixa de mundo do tile, começando em `((X-20)·32768, (Y-18)·32768)`
- [x] Câmera livre com WASD, Q/E, Shift para acelerar, arrastar para olhar e roda para aproximar
- [x] Um mapa com `TerrainScale` quebrado cai no fallback por `MapX/MapY` em vez de falhar

## Comments

Resolvido na branch de integração `zone-builder`. Branch `zb/03-terreno-sem-textura`. Contagem e ordem dos índices do terreno iguais ao UE2-Studio em 22_22, 23_22 e 17_22_Classic; faixa de mundo do tile testada. Fallback por MapX/MapY verificado só em caso sintético (nenhum mapa do Fafurion tem TerrainScale quebrado). Manual: comparação visual na janela do UE2-Studio.
