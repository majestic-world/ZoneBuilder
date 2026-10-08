# 07: Static meshes no viewport e no picking

**What to build:** ao abrir o mapa, os atores de static mesh aparecem no viewport, ainda sem textura, e o clique acerta telhados, pontes e objetos. Os atores cobertos são `StaticMeshActor`, `MovableStaticMeshActor`, `L2MovableStaticMeshActor`, `Mover` e `L2NMover`. Isso cobre:
- a resolução de imports entre pacotes, procurando por nome nas pastas de assets do cliente com cache por pacote;
- o leitor de `StaticMesh`;
- o transform `Location - PrePivot`, com rotação GLM-euler e escala `DrawScale3D·DrawScale`.

**Blocked by:** 04 (Clique no terreno devolve `x y z`)

**Status:** ready-for-agent

- [ ] Um tile de cidade mostra construções e objetos nas mesmas posições, rotações e escalas que o UE2-Studio
- [ ] O clique num telhado devolve o `x y z` do telhado, mais alto que o terreno abaixo dele
- [ ] Um pacote `.usx` ausente gera um aviso com o nome do pacote, e o resto do mapa abre
