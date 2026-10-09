# 01: Terreno como camada, com fallback

**What to build:** o terreno passa a obedecer à regra de alcance de camadas como um bloco: ele conta quando o span do terreno sob o contorno cruza `[zmin − GroundReach, zmax + GroundReach]`, ou quando nenhuma peça de BSP ou mesh não excluída alcança a faixa (spec D1). `Measure` guarda o span do terreno no `Profile`; o teste "alguma BSP/mesh alcança" sai de buscas binárias num índice só das peças que não são terreno. A regra vale em `reaches`, ou seja, em `Fit`, `ground`, `others`, `Classify`, no histograma e nos avisos, julgada uma vez a partir da faixa de antes do clique, como hoje. Quando fica de fora, o terreno aparece em `Others`, como qualquer outra camada.

**Blocked by:** None (can start immediately)

**Status:** needs-triage

- [ ] Teste do seam (torre): terreno a 0 e piso de BSP a 15.000; `Fit` a partir de `[14744, 15256]` mantém `14744 … 15256`, `Classify` sem chão abaixo do piso nem aviso, e o terreno em `Others`
- [ ] Teste do seam (caverna): terreno a 0 e piso de BSP a −3000; `Fit` a partir da faixa da caverna não sobe o topo até o terreno
- [ ] Teste do seam (morro): os cantos a 0 e um pico de terreno a 3000; `Fit` a partir de `[−256, 256]` ainda põe o topo acima do pico
- [ ] `TestPeakFarFromTheVerticesSetsTheTop` passa sem mudar a expectativa; o comentário dele cita o fallback
- [ ] Os testes de `coverage` de hoje continuam passando
- [ ] `Classify` com `-fps` em 22_22 e em 23_18, arrastando a seta Z: p99 abaixo de 2 ms
- [ ] No app, no 23_18: o polígono fechado no topo da torre nasce com o piso perto do topo, não a −5376

## Comments
