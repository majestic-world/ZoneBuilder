# 06: BSP no viewport e no picking

**What to build:** ao abrir o mapa, as superfícies BSP do Level aparecem no viewport, ainda sem textura, e o clique acerta paredes, pisos e interiores BSP. Isso cobre:
- o `Model`: vectors, points, nodes, surfs e verts;
- o fan por nó;
- os gates de licensee do Level (segundo array de atores) e das superfícies (campo extra);
- o pulo das superfícies invisíveis, de portal e de backdrop;
- Möller–Trumbore com early-out por AABB no `Pick`.

**Blocked by:** 04 (Clique no terreno devolve `x y z`)

**Status:** ready-for-agent

- [ ] Um tile com interior BSP mostra as superfícies nas mesmas posições que o UE2-Studio
- [ ] Superfícies invisíveis, de portal e de backdrop não aparecem e não são atingidas pelo clique
- [ ] O clique num piso BSP devolve o `x y z` do piso, e o acerto mais próximo vence quando terreno e BSP se sobrepõem
- [ ] Os filtros de região maior que 2 tiles e fora do mapa removem os quads de backdrop, como no UE2-Studio
