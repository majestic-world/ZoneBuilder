# Z do servidor: o chão do servidor fica 32 unidades acima da superfície do cliente

O spec supunha que o Z do mapa do cliente é o Z do servidor, 1:1. Medido, não é: o chão que o servidor usa (a geodata do datapack, `gameserver/geodata/X_Y.l2j`, que é a altura em que ele põe NPCs e jogadores e a que o `//pos` imprime) fica cerca de 32 unidades acima da superfície do terreno do cliente. A superfície vem da fórmula do UE2 (`Location.Z + (h − 32768) · TerrainScale.Z / 256`), a mesma do UE2-Studio, que o ticket 03 conferiu contra ele. X e Y batem sem deslocamento. Por isso a cena guarda a geometria no espaço do cliente e converte na fronteira: `scene.ServerZOffset = 32`, `scene.ToServer` e `scene.FromServer`. O `Hit` do `Pick` já sai em coordenadas do servidor. Tudo que vem do servidor e precisa encontrar a geometria (zonas, um `x y z` digitado) passa por `FromServer`.

## Medição (2026-10-08, cliente Fafurion, datapack Majestic)

- **Método:** um ray vertical no centro de células de geodata de 16×16 unidades, comparando a superfície do terreno do cliente com a altura da geodata. Entram só as células de 1 camada (blocos plano e complexo da geodata; os multicamada ficam de fora), e só onde o terreno é visível.
- **Terreno plano** (desnível menor que 1 unidade em 32 unidades), passo de 128 unidades, 4.019.245 células em 150 tiles com mapa e geodata:
  - diferença cliente − servidor: média −29,8, mediana −31,35, p5 −32,7, p95 −19,9;
  - 3.323.441 células, 83%, caem em [−36, −28);
  - nos tiles de oceano (16_14, 18_11, 20_10, 16_22, 18_13…), a diferença é constante dentro de cada tile, entre −31,3 e −32,7.
  - A cauda acima de −28 vem da quantização da geodata em passos de 8 e das células em que um mesh ou BSP ainda sem leitura (tickets 06/07) fica sobre o terreno.
- **Giran (22_22), terreno inteiro**, passo de 256 unidades, 14.846 células: mediana −28,5, p10 −33,5, p90 −14,0.
  - Um ajuste `Δ = c + a·∂z/∂x + b·∂z/∂y` dá inclinações perto de 0 em 7 tiles (22_22, 23_22, 21_22, 22_21, 17_22, 20_18, 22_23). Logo não há deslocamento em X/Y, só em Z.
- **Spawns do datapack:** a geodata bate com a posição de spawn das NPCs a ±8 nos pontos de 1 camada (`spawn/22_22.xml`). As 10 NPCs dos portões de Giran, que estão em terreno aberto, dão Δ de −27,4 a −31,1. Com `ServerZOffset` somado, ficam entre +0,9 e +4,6. Esses 10 pontos são o teste `TestPickLandsOnGiranServerGround`.
- **22_22 ou 22_22_Classic:** os 2 mapas têm a mesma posição e a mesma escala, os mesmos quads visíveis nas 987.424 amostras, e 62.073 das 65.536 alturas são iguais. Nas amostras (passo de 32) em que os 2 picks dão Z diferente, o 22_22 fica mais perto da geodata em 50.214 delas e o Classic em 7.089. **O servidor corresponde ao `22_22`** (não Classic), e o app continua abrindo `22_22` por padrão.

A sonda e a saída dela estão em `zone-builder-notes/04-evidence/`. Elas não fazem parte do repositório, porque leem a geodata, que fica fora do cliente.

## Consequences

- O `ServerZOffset` é empírico. Ele vale para a geodata desse datapack. Uma geodata gerada por outra ferramenta pode ter outra constante: o `l2-geodata-toolkit` calibra a dele em +43,35.
- Qualquer coisa que desenhe dados do espaço do servidor precisa converter com `FromServer` antes de `ToRender`: o overlay de zonas, o "ir para x y z" da câmera e os pontos de restart. Sem isso, tudo aparece 32 unidades alto demais.
- Para o `zmin`/`zmax` das zonas, o Z de um clique já sai no espaço do servidor. A UI mostra esse valor sem conversão.
