# Terreno como camada, julgado em bloco, com fallback

Substitui o parágrafo **Camadas** do [ADR 0004](0004-chao-medido-e-avisos-sem-bloqueio.md).

No 23_18, um polígono fechado com os 16 vértices no alto da Tower of Insolence (BSP, z ≈ 10.197) nascia com `z −5376 … 10453` "pelo chão da área". A regra do ADR 0004 dizia que o terreno sempre conta, e só BSP e mesh precisavam ficar a até `GroundReach = 1024` da faixa. O terreno a −5120 sob a torre puxava o piso até o chão do mapa. A mesma regra valia no `Classify`, nos avisos, na régua e em "Piso ao chão" e "Topo ao chão": uma zona correta no topo da torre acusava "chão abaixo do piso" por causa do terreno.

## Decisão

O terreno segue a regra de alcance, como BSP e mesh, mas julgado como **um bloco**: todo o terreno sob o contorno conta, ou nada dele conta.

```
terrenoConta(zmin, zmax) =
    span do terreno sob o contorno cruza [zmin − GroundReach, zmax + GroundReach]
    OU nenhuma peça de BSP/mesh (não excluída) alcança [zmin, zmax]
```

- **Bloco.** O terreno é uma superfície contínua: se uma parte dele alcança a faixa, todo o terreno sob o contorno é a mesma camada. Assim um morro de 3000 no meio de um retângulo do vale continua contando, porque os cantos estão no terreno.
- **Fallback.** Se nenhuma peça de BSP ou mesh alcança a faixa, o terreno conta de qualquer jeito. Uma faixa longe de tudo (shape criado fora dos tiles, faixa arrastada para longe) continua sendo ajustada pelo terreno em "Recalcular pelo chão".
- **Onde vale.** No julgamento por peça (`coverage.reaches`), e portanto em `Fit`, `Classify`, "outras camadas", histograma da régua e avisos. O veredito é calculado uma vez por chamada e sai em `Report.Terrain`. Como no ADR 0004, nos ajustes ele é julgado **uma vez**, a partir da faixa de antes do clique.
- **Outras camadas.** Quando fica de fora, o terreno entra em "outras camadas" como uma camada só, com o span inteiro dele.
- **Custo.** `Measure` guarda o span do terreno no `Profile`. "Alguma peça de BSP/mesh alcança" é uma busca binária em `byLow` mais o máximo prefixo de `zhi` das peças sem exclusão; as peças sob uma exclusão são testadas uma a uma. Medido com `-fps` e timer QPC, o p99 do `Classify` ficou igual antes e depois: 0,092 ms na zona do relato e 1,822 ms no tile inteiro do 23_18, 1,600 ms no tile inteiro do 22_22, abaixo dos 2 ms do ADR 0004.

| Caso | Antes | Depois |
| --- | --- | --- |
| Polígono no topo da torre (23_18) | −5376 … 10340 | 8464 … 10340: o terreno vira outra camada |
| Retângulo no vale com morro de 3000 | morro conta | morro conta (o terreno alcança pelos cantos) |
| Caverna de BSP 3000 abaixo do terreno | o terreno estica o topo até a superfície | o terreno vira outra camada |
| Faixa longe de tudo, "Recalcular pelo chão" | o terreno ajusta | o terreno ajusta (fallback) |
| Andar de prédio 600 acima do terreno | o terreno conta | o terreno conta (dentro do alcance) |

Para os casos em que nenhuma regra de camadas acerta (o andar de prédio ao alcance do terreno, os pisos internos de uma torre), o inspetor ganhou o seletor **"Faixa nova: Chão da área | Pontos clicados"**. Em pontos clicados, um polígono, retângulo ou círculo novo nasce com `menorZ − folga … maiorZ + folga` dos vértices, sem medir o chão. O tile inteiro sempre usa o chão da área.

## Descartadas

- **Terreno peça a peça**, como BSP e mesh: só contariam as células do terreno a até 1024 da faixa. Corta morros: no vale com um pico de 3000, o topo pararia no anel do morro que ainda alcança (1756 no teste `TestHillTerrainCountsAsOneBlock`), e o pico ficaria como "outra camada" dentro do próprio retângulo.
- **Manter o terreno sempre contando** e resolver o topo da torre só com o modo pontos clicados: a zona correta continuaria com aviso de terreno e "Piso ao chão" continuaria a jogá-la no chão do mapa.

## Consequences

- No modo chão da área, os pisos internos de BSP de uma estrutura alta, a até `GroundReach` da faixa, continuam puxando o piso. E, depois da criação, podem gerar um aviso "chão abaixo do piso": o `Fit` julga as camadas a partir da faixa dos pontos clicados, e o `Classify` julga de novo a partir da faixa ajustada, que alcança mais um andar (a cascata do ADR 0004). Medido no 23_18: um polígono de 6 vértices no topo (z 10083) nasce com `8464 … 10340` e fica com 1 aviso de folga do piso −990, de um piso interno a z 7474. A saída explícita é "Faixa nova: Pontos clicados".
- Uma zona sobre uma ponte de BSP a menos de 1024 do rio não muda: o terreno está ao alcance. O terreno só sai quando está a mais de 1024 da faixa **e** existe BSP ou mesh ao alcance; o Z dele aparece em "outras camadas".
- Shapes que já existem não mudam: a regra só age em ajustes e na criação. Uma zona criada com a regra antiga se corrige com Base, Altura e "Piso ao chão", que agora respeita a torre.
- A escolha de "Faixa nova" vale só na sessão, como a folga Z.
