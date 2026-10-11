# Chão enterrado sob a faixa é outra camada

Complementa a regra de alcance do [ADR 0004](0004-chao-medido-e-avisos-sem-bloqueio.md) e do [ADR 0006](0006-terreno-como-camada.md) para BSP e mesh, e resolve a primeira consequência do ADR 0006.

No 22_22, um polígono desenhado no pátio do castelo de Giran (piso de pedra, static mesh com colisão em z 3494,5) nascia com `1990 … 3751` "pelo chão da área". Sob o pátio ficam os salões do castelo: pisos de mesh com colisão de 2400 a 3300, um vão fechado em 3348 e musgo sem colisão em 2393. Todos ficam a até `GroundReach = 1024` da faixa dos pontos clicados (`3238 … 3750`), então contavam, e o piso ia para 2246 − 256. "Piso ao chão" afundava mais em cascata: 823 e depois 750. Onde o chão é terreno, a faixa já nascia certa.

## Decisão

Um piso de BSP ou mesh é **enterrado**, e vira outra camada, quando:

```
enterrado(peça, zmin, zmax, apoio) =
    zhi da peça < apoio
    E na coluna da peça há um piso de BSP ou mesh, mais de LayerGap acima dela, com Z em [zmin, zmax]
```

- **Coluna.** É a mesma do `stack` (um ponto dentro de cada peça, `Profile.inner`), medida uma vez em `Measure`. Cada peça guarda os Z dos pisos acima dela na coluna (`Profile.coverZ`); o julgamento por faixa é uma busca binária.
- **Apoio.** No `Classify` (inspetor, régua, linha do chão nas arestas, outras camadas), o apoio é `zmin`. No `Fit` (criação, "Recalcular pelo chão", "Piso ao chão", "Topo ao chão"), é `zmin + folga − LayerGap`: o chão onde a faixa se apoia, porque toda faixa sugerida ou ajustada é "chão − folga". O `− LayerGap` evita enterrar o próprio chão clicado, cujo Z o vértice arredonda.
- **O terreno não enterra.** Só um piso de BSP ou mesh cobre outro. Uma zona no solo mede o chão como antes, inclusive o espelho d'água de BSP em −3748 sob o terreno de Giran, que continua puxando o piso das zonas de rua ao alcance dele.
- **Chão descoberto conta.** Um piso abaixo da faixa sem nada da faixa por cima (uma rampa que desce para um pátio aberto) segue a regra de alcance.

| Caso | Antes | Depois |
| --- | --- | --- |
| Polígono no pátio de Giran (22_22), pontos em 3494 | 1990 … 3751; "Piso ao chão" 823 | ~3230 … 3751; "Piso ao chão" mantém |
| Quadrados com vértices na Tower of Insolence (23_18) | piso 8576, 8744, 8756 | piso 9786, 9826, 9826 |
| 284 quadrados aleatórios no 22_22 e no 23_18, faixa pelo chão sob os vértices | — | dos 141 com os 4 vértices no terreno, 139 iguais e 2 com o piso 15 e 47 mais alto, por estruturas dentro deles |
| Andar térreo de BSP sob um mezanino dentro da faixa | conta | conta (está no apoio) |

## Custo

`Measure` grava a cobertura no laço do `stack`, que já existia. Para `others` não ler as peças fora de ordem, `byLow` carrega `zlo`, `zhi` e a marca de exclusão, e `coveredLow` lista só as peças cobertas, com o menor e o maior Z da cobertura. No tile inteiro do 22_22, varrendo 400 faixas de 1500 de altura, o `Classify` médio foi de cerca de 1,6 ms para 1,9 ms (profile), e o p99 de 3,0–3,5 ms para 4,0–4,5 ms (timer com resolução de 0,5 ms). O aumento vem de mais faixas terem "outras camadas" para listar. Zonas comuns seguem em décimos de ms.

## Descartadas

- **Só o chão mais alto de cada coluna conta.** Enterraria o andar térreo onde o usuário clicou, sob um mezanino dentro da faixa.
- **Tirar meshes sem colisão do chão** (`Blocks`). Resolve o musgo, mas os salões têm colisão: no pátio, o piso ainda iria para cerca de 2150.
- **Terreno também enterra.** Mudaria zonas no solo: o espelho d'água sob o terreno de Giran deixaria de puxar, e o piso de muitas zonas de rua do 22_22 subiria (de −4004 para o terreno − 256).

## Consequences

- Os pisos dentro da folga, entre o piso e o apoio, continuam contando. No pátio de Giran, o vão fechado em 3348 aparece como chão dentro da faixa, com folga do piso positiva (+112 a +121).
- O `Fit` e o `Classify` usam apoios diferentes, então logo depois de um ajuste o inspetor pode mostrar como chão um piso dentro da folga que o `Fit` ignorou. Ele está dentro da faixa: não fura o piso.
- Uma zona que já existe com o piso nos salões continua assim: "Piso ao chão" julga a partir do piso atual, e os salões dentro da faixa contam. Para corrigir, use Base ou crie de novo.
- A cobertura é julgada num ponto por peça, como a contagem de camadas: o erro fica limitado a 1 triângulo na borda do piso de cima.
- Os testes `TestFloorBuriedUnderTheCourtyardDoesNotPullIt` e `TestFloorTheRangeStandsOnIsNotBuried` cobrem a regra, e `TestGiranCourtyardRangeStaysOnTheCourtyard` o pátio no cliente real.
