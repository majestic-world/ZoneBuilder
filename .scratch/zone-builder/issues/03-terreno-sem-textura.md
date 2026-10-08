# 03: Terreno sem textura no viewport

**What to build:** o usuário aponta o app para a pasta do cliente, escolhe um tile `X_Y` e vê o terreno daquele mapa no viewport, sem textura, voando com a câmera livre do UE2-Studio. Isso cobre:
- as propriedades com tag (com o preâmbulo `RF_HAS_STACK`);
- o `TerrainInfo`;
- o heightmap G16;
- `TerrainScale`, `QuadVisibilityBitmap`, `EdgeTurnBitmap` e o fallback por `MapX/MapY`.

Este ticket abre o seam Cena (`Load`). A cena guarda coordenadas absolutas e expõe uma origem de rebase para o renderizador.

**Blocked by:** 01 (Spike de renderização Gio + ANGLE), 02 (Abrir pacotes e listar exports)

**Status:** ready-for-agent

- [ ] Escolher a pasta do cliente e o tile Giran abre o terreno no lugar certo, com os quads invisíveis faltando onde o UE2-Studio também não desenha
- [ ] O terreno ocupa a faixa de mundo do tile, começando em `((X-20)·32768, (Y-18)·32768)`
- [ ] Câmera livre com WASD, Q/E, Shift para acelerar, arrastar para olhar e roda para aproximar
- [ ] Um mapa com `TerrainScale` quebrado cai no fallback por `MapX/MapY` em vez de falhar
