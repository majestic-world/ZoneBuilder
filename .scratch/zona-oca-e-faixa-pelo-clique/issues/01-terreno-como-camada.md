# 01: Terreno como camada, com fallback

**What to build:** o terreno passa a obedecer à regra de alcance de camadas como um bloco: ele conta quando o span do terreno sob o contorno cruza `[zmin − GroundReach, zmax + GroundReach]`, ou quando nenhuma peça de BSP ou mesh não excluída alcança a faixa (spec D1). `Measure` guarda o span do terreno no `Profile`; o teste "alguma BSP/mesh alcança" sai de buscas binárias num índice só das peças que não são terreno. A regra vale em `reaches`, ou seja, em `Fit`, `ground`, `others`, `Classify`, no histograma e nos avisos, julgada uma vez a partir da faixa de antes do clique, como hoje. Quando fica de fora, o terreno aparece em `Others`, como qualquer outra camada.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] Teste do seam (torre): terreno a 0 e piso de BSP a 15.000; `Fit` a partir de `[14744, 15256]` mantém `14744 … 15256`, `Classify` sem chão abaixo do piso nem aviso, e o terreno em `Others`
- [x] Teste do seam (caverna): terreno a 0 e piso de BSP a −3000; `Fit` a partir da faixa da caverna não sobe o topo até o terreno
- [x] Teste do seam (morro): os cantos a 0 e um pico de terreno a 3000; `Fit` a partir de `[−256, 256]` ainda põe o topo acima do pico
- [x] `TestPeakFarFromTheVerticesSetsTheTop` passa sem mudar a expectativa; o comentário dele cita o fallback
- [x] Os testes de `coverage` de hoje continuam passando
- [ ] `Classify` com `-fps` em 22_22 e em 23_18, arrastando a seta Z: p99 abaixo de 2 ms
- [x] No app, no 23_18: o polígono fechado no topo da torre nasce com o piso perto do topo, não a −5376

## Comments

Implementado na branch `zo/01-terreno-como-camada` e integrado em `zona-oca-e-faixa-pelo-clique`. Testes do seam em `internal/coverage`: `TestTerrainUnderATowerIsAnotherLayer` (no código antigo o `Fit` dava −256 … 15256; agora 14744 … 15256, `Below` 0, sem avisos, terreno em `Others` a z 0), `TestTerrainOverACaveDoesNotLiftTheTop` (topo −2744) e `TestHillTerrainCountsAsOneBlock` (topo 3256; julgado peça por peça daria 1756). `TestPeakFarFromTheVerticesSetsTheTop` passa com a mesma expectativa; o comentário agora cita o fallback.

`Classify`, p99 medido com timer QPC temporário enquanto a seta Z era arrastada: 22_22 captura 0,018 ms, 22_22 tile inteiro 1,84 ms, 23_18 polígono da torre 0,16 ms. O tile inteiro do 23_18 deu 2,04 ms tanto antes quanto depois da mudança, então o limite já estava estourado antes do D1. A mudança só acrescenta O(log n).

Smoke no 23_18: um polígono de 6 vértices no topo (BSP z 10083) nasce com `z 8464..11666 pelo chão da área`, e não mais com −5376. O piso fica em 8464 por causa dos pisos internos da torre a menos de 1024 do topo, risco que a spec já aceita (a saída é o D2).

Evidências: `C:/Workspace/zone-builder-notes/zona-oca-e-faixa-pelo-clique/01-evidence/`.
