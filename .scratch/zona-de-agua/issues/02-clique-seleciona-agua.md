# 02: Clique seleciona a água

**What to build:** sem ferramenta armada, um clique esquerdo na água seleciona o corpo d'água: o volume sob o clique mais os volumes encostados nele com o mesmo topo, também entre tiles carregados. Ctrl+clique alterna um volume avulso. Esc ou clique fora da água limpa a seleção. O teste é `World.PickWater(ray, maxDist)`: o raio é recortado pelos planos de cada volume convexo, e vence a menor entrada até a distância do `Pick` normal. Assim, clicar na superfície, no fundo visto através da água ou numa fonte de static mesh seleciona; clicar na margem seca não seleciona (spec D3). Os volumes selecionados aparecem como `render.ZoneShape` na cor de `zone.Water`, com a pegada, o fundo e o topo passados por `ToServer`, para que o overlay os desenhe exatamente sobre o volume. O status mostra `Água: N volumes · topo T (servidor T') · exata|aproximada`, e água sem volume dá `Esta água não tem WaterVolume no mapa`. O estado fica em `cmd/zonebuilder/water.go`, por `(tile, export)`, e perde os volumes de tiles descarregados.

**Blocked by:** 01 (Volumes de água na cena)

**Status:** resolved

- [x] Teste do seam, raio × poliedro convexo: entra pelo topo (entrada = distância até o topo), passa ao lado (sem acerto), nasce dentro (entrada 0), raspa uma aresta (acerto estável) e um volume atrás do chão, com entrada além de `maxDist`, não é escolhido
- [x] Teste do seam: 3 caixas encostadas com o mesmo topo formam 1 corpo, e uma 4ª encostada com topo 100 acima fica fora
- [x] Teste do seam: um volume `Unsupported` nunca é escolhido
- [x] No app, em 22_22, clicar no mar de Giran destaca os 8 volumes e o status diz `8 volumes`, `topo −3780` e `exata`
- [x] No app, clicar na margem seca ao lado não seleciona; Esc limpa
- [x] No app, com uma ferramenta de zona armada, o clique na água continua desenhando o shape e não seleciona água
- [x] Ctrl+clique num volume do corpo o tira da seleção, e Ctrl+clique de novo o devolve

## Comments

Implementado na branch `za/02-clique-seleciona-agua` e integrado em `zona-de-agua`. Testes: `TestPickWaterClipsTheRayByTheVolumePlanes` (com o caso da cunha acrescentado no code review), `TestWaterBodyJoinsTouchingVolumesWithTheSameTop` (uma caixa em outra cena, para cobrir corpos entre tiles), `TestPickWaterNeverChoosesAnUnsupportedVolume`. Smoke com capturas: 8 prismas e `Água: 8 volumes · topo -3780 (servidor -3810) · exata`; margem seca e Esc limpam; ferramenta armada desenha o vértice; Ctrl+clique vai de 8 a 7 e volta a 8. A mensagem `Esta água não tem WaterVolume no mapa` e a guarda das alças de vértice foram conferidas no smoke do ticket 04.

Evidências: `C:/Workspace/zone-builder-notes/zona-de-agua/02-evidence/`.

Mudança pedida pelo usuário depois da entrega (decisão 2 da spec): o clique seleciona só o volume clicado, e o Ctrl+clique põe ou tira outros; o clique direito num volume fora da seleção o seleciona sozinho. `World.WaterBody` e o teste do corpo d'água saíram. Smoke em 22_22: clique dá `Água: 1 volume`, Ctrl+clique em outro dá `2 volumes`, Ctrl+clique de novo volta a `1 volume`, e o clique direito num volume não selecionado abre o menu com só ele selecionado.
