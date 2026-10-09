# 06: Números ao vivo e resumo da zona na janela de altura

**What to build:** arrastar a seta Z, usar subir/descer ou digitar `zmin zmax` atualiza a cobertura a cada frame, só reclassificando o perfil já medido, sem medir de novo (spec D2). A janela de altura mostra, abaixo de piso, topo e altura, a cobertura da zona inteira: o chão mais baixo e o mais alto sob a zona, as folgas e as áreas somadas dos shapes incluídos, com a indicação "soma dos shapes". Se a reclassificação de uma zona do tamanho de um tile passar de 2 ms, o perfil passa a guardar a área acumulada por Z.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] No cliente real, arrastar a seta Z sobre uma zona do tamanho de um tile muda os números a cada frame, e o log de `-fps` não piora em relação a uma zona pequena
- [ ] A janela de altura mostra a cobertura da zona e muda junto com a seta
- [ ] Desfazer uma mudança de faixa volta os números sem medir de novo
