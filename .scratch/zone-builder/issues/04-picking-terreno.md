# 04: Clique no terreno devolve `x y z`

**What to build:** passar o mouse e clicar sobre o terreno mostra a posição de mundo na barra de status, em coordenadas do servidor, sem conversão. Este ticket cria o `Pick(ray) → Hit` do seam Cena: ray a partir do cursor e DDA no grid do terreno. Ele também confirma que o Z do mapa bate 1:1 com o do servidor.

**Blocked by:** 03 (Terreno sem textura no viewport)

**Status:** resolved

- [x] A barra de status mostra `x y z` sob o cursor, arredondados para inteiro
- [x] Teste do seam Cena: em pontos de referência de Giran com coordenadas anotadas via `//pos` no jogo, `Pick` de um ray vertical devolve `x y z` a menos de 16 unidades
- [x] O clique fora do terreno não devolve acerto e não quebra nada

## Comments

Resolvido na branch de integração `zone-builder`. Branch `zb/04-picking-terreno`. O Z do servidor fica 32 acima da superfície do cliente (ADR `docs/adr/0003-z-do-servidor-32-acima-do-cliente.md`); `Hit.Pos` sai em coordenadas do servidor. Teste com 10 spawns do datapack em Giran 22_22, Δz ≤ 4,6. Manual: conferir com `//pos` no jogo.
