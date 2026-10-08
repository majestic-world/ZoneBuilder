# 06: BSP no viewport e no picking

**What to build:** ao abrir o mapa, as superfícies BSP do Level aparecem no viewport, ainda sem textura, e o clique acerta paredes, pisos e interiores BSP. Isso cobre:
- o `Model`: vectors, points, nodes, surfs e verts;
- o fan por nó;
- os gates de licensee do Level (segundo array de atores) e das superfícies (campo extra);
- o pulo das superfícies invisíveis, de portal e de backdrop;
- Möller–Trumbore com early-out por AABB no `Pick`.

**Blocked by:** 04 (Clique no terreno devolve `x y z`)

**Status:** resolved

- [x] Um tile com interior BSP mostra as superfícies nas mesmas posições que o UE2-Studio
- [x] Superfícies invisíveis, de portal e de backdrop não aparecem e não são atingidas pelo clique
- [x] O clique num piso BSP devolve o `x y z` do piso, e o acerto mais próximo vence quando terreno e BSP se sobrepõem
- [x] Os filtros de região maior que 2 tiles e fora do mapa removem os quads de backdrop, como no UE2-Studio

## Comments

Resolvido na branch de integração `zone-builder`. Branch `zb/06-bsp`. Superfícies, triângulos e checksum de posições iguais ao UE2-Studio em 5 mapas; picking em pisos BSP de Giran a menos de 16 unidades. Manual: comparação visual na janela do UE2-Studio e `//pos` no jogo.
