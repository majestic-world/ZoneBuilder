# 04: BSP e static meshes como chão

**What to build:** a medição da cobertura passa a contar como chão, além do terreno, as faces de BSP e de static mesh voltadas para cima (`normal.z ≥ 0,5`). Com o botão Static meshes desligado, meshes não entram: o que se vê é o que se mede (decisão adotada). O inspetor mostra o maior número de camadas numa coluna do shape (terreno, piso de prédio, ponte), e uma ponte acima do topo conta como chão acima do topo.

**Blocked by:** 01

**Status:** resolved

- [x] Teste do seam: uma ponte sobre o terreno dá 2 camadas, e a ponte acima do topo entra na área acima do topo
- [x] Teste do seam: uma parede (face vertical) e um teto (face para baixo) não contam como chão
- [x] Teste com o cliente real: com meshes visíveis, numa coluna sobre um mesh, o maior Z dos triângulos de chão é igual ao Z do `Pick` vertical quando a face atingida é voltada para cima
- [x] No app, alternar o botão Static meshes refaz a medição de um shape sobre uma ponte ou prédio, e o número de camadas muda

## Comments

Implementado na branch `cv/04-bsp-meshes-como-chao` e integrado em `cobertura-vertical`. Seam: ponte em 2 camadas e acima do topo; `TestBSPWallsAndCeilingsAreNotFloor` e `TestFloorTopOverMeshesMatchesTheVerticalPick` no cliente real. Smoke em Giran: 6 camadas com meshes visíveis, 2 com meshes ocultos. Perfil de um tile com meshes: cerca de 98 ms.

Evidências: `C:/Workspace/zone-builder-notes/cobertura-vertical/04-evidence/`.
