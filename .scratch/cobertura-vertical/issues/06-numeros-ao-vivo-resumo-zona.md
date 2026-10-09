# 06: Números ao vivo e resumo da zona na janela de altura

**What to build:** arrastar a seta Z, usar subir/descer ou digitar `zmin zmax` atualiza a cobertura a cada frame, só reclassificando o perfil já medido, sem medir de novo (spec D2). A janela de altura mostra, abaixo de piso, topo e altura, a cobertura da zona inteira: o chão mais baixo e o mais alto sob a zona, as folgas e as áreas somadas dos shapes incluídos, com a indicação "soma dos shapes". Se a reclassificação de uma zona do tamanho de um tile passar de 2 ms, o perfil passa a guardar a área acumulada por Z.

**Blocked by:** 01

**Status:** resolved

- [x] No cliente real, arrastar a seta Z sobre uma zona do tamanho de um tile muda os números a cada frame, e o log de `-fps` não piora em relação a uma zona pequena
- [x] A janela de altura mostra a cobertura da zona e muda junto com a seta
- [x] Desfazer uma mudança de faixa volta os números sem medir de novo

## Comments

Implementado na branch `cv/06-numeros-ao-vivo-resumo-zona` e integrado em `cobertura-vertical`. Smoke com `-fps`: arrastar a seta Z numa zona do tamanho de um tile ficou em cerca de 65 q/s, igual à zona pequena. Reclassificar leva de 1,2 a 1,4 ms em média com meshes, abaixo de 2 ms, então a tabela por Z não foi criada. Desfazer volta os números sem medir de novo. `coverage.Sum` tem teste no seam.

Evidências: `C:/Workspace/zone-builder-notes/cobertura-vertical/06-evidence/`.
