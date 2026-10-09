# 02: Clique seleciona a água

**What to build:** sem ferramenta armada, um clique esquerdo na água seleciona o corpo d'água: o volume sob o clique mais os volumes encostados nele com o mesmo topo, também entre tiles carregados. Ctrl+clique alterna um volume avulso. Esc ou clique fora da água limpa a seleção. O teste é `World.PickWater(ray, maxDist)`: o raio é recortado pelos planos de cada volume convexo, e vence a menor entrada até a distância do `Pick` normal. Assim, clicar na superfície, no fundo visto através da água ou numa fonte de static mesh seleciona; clicar na margem seca não seleciona (spec D3). Os volumes selecionados aparecem como `render.ZoneShape` na cor de `zone.Water`, com a pegada, o fundo e o topo passados por `ToServer`, para que o overlay os desenhe exatamente sobre o volume. O status mostra `Água: N volumes · topo T (servidor T') · exata|aproximada`, e água sem volume dá `Esta água não tem WaterVolume no mapa`. O estado fica em `cmd/zonebuilder/water.go`, por `(tile, export)`, e perde os volumes de tiles descarregados.

**Blocked by:** 01 (Volumes de água na cena)

**Status:** ready-for-agent

- [ ] Teste do seam, raio × poliedro convexo: entra pelo topo (entrada = distância até o topo), passa ao lado (sem acerto), nasce dentro (entrada 0), raspa uma aresta (acerto estável) e um volume atrás do chão, com entrada além de `maxDist`, não é escolhido
- [ ] Teste do seam: 3 caixas encostadas com o mesmo topo formam 1 corpo, e uma 4ª encostada com topo 100 acima fica fora
- [ ] Teste do seam: um volume `Unsupported` nunca é escolhido
- [ ] No app, em 22_22, clicar no mar de Giran destaca os 8 volumes e o status diz `8 volumes`, `topo −3780` e `exata`
- [ ] No app, clicar na margem seca ao lado não seleciona; Esc limpa
- [ ] No app, com uma ferramenta de zona armada, o clique na água continua desenhando o shape e não seleciona água
- [ ] Ctrl+clique num volume do corpo o tira da seleção, e Ctrl+clique de novo o devolve

## Comments
