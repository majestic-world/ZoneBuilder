# 04: Clique no terreno devolve `x y z`

**What to build:** passar o mouse e clicar sobre o terreno mostra a posição de mundo na barra de status, em coordenadas do servidor, sem conversão. Este ticket cria o `Pick(ray) → Hit` do seam Cena: ray a partir do cursor e DDA no grid do terreno. Ele também confirma que o Z do mapa bate 1:1 com o do servidor.

**Blocked by:** 03 (Terreno sem textura no viewport)

**Status:** ready-for-agent

- [ ] A barra de status mostra `x y z` sob o cursor, arredondados para inteiro
- [ ] Teste do seam Cena: em pontos de referência de Giran com coordenadas anotadas via `//pos` no jogo, `Pick` de um ray vertical devolve `x y z` a menos de 16 unidades
- [ ] O clique fora do terreno não devolve acerto e não quebra nada
