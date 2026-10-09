# 03: Volume vira zona de água, e ADR 0005

**What to build:** o pacote `internal/water` (novo, sem GL e sem Gio) transforma volumes de água em planos de zona: `Compile(volumes, existing) []Plan`. Os volumes são agrupados pelo topo no Z do servidor, com 1 zona por topo, porque o servidor usa o maior `zmax` da zona como superfície. Cada volume vira 1 polígono: a envoltória convexa XY dos vértices das faces, sem pontos colineares, arredondada, com o próprio `zmin = round(fundo) + ServerZOffset` e `zmax = round(topo) + ServerZOffset`. O tipo é `water`, sem parâmetros, e o nome é `[X_Y_<volume de menor nome>]`. Um nome que já existe no projeto não gera comandos: o plano devolve a zona existente. Os avisos de D7 (aproximada, água sobreposta, fora do tile) saem no plano. A constante `ServerZOffset` segue a decisão aprovada da spec D6, e o ADR 0005 registra a medição do datapack, a contradição com o ADR 0003 e a consequência visível: a zona compilada aparece 62 abaixo da superfície no viewport com −30.

**Blocked by:** 01 (Volumes de água na cena)

**Status:** ready-for-agent

- [ ] Teste com o cliente real (pulado sem `ZB_CLIENT`): os 11 volumes de topo −3780 de 22_24 dão 1 zona com 11 polígonos iguais, ponto a ponto e em `zmin`/`zmax`, aos shapes `[22_24_water1…9]`, `[22_24_water12]` e `[22_24_water13]` do datapack, com os valores copiados no teste
- [ ] Teste com o cliente real: `WaterVolume26` e `WaterVolume27` de 22_24 (encostados, topos −3746 e −5529) dão 2 zonas
- [ ] Teste do seam: 2 topos dão 2 zonas, e cada polígono mantém o próprio `zmin`
- [ ] Teste do seam: hexaedro com parede inclinada vira pegada convexa sem pontos colineares e com o aviso "aproximada"
- [ ] Teste do seam: um volume que cruza em XY e Z outro volume vivo, de topo diferente e fora da seleção, gera o aviso de água sobreposta; encostar sem cruzar não gera
- [ ] Teste do seam: com o nome já no projeto, o plano não tem comandos e aponta para a zona existente
- [ ] Teste do seam: a zona gerada não tem nenhum problema de `Document.Problems()` (os comandos aplicados num documento vazio compilam sem `BlockedError`)
- [ ] `docs/adr/0005-...md` escrito com a tabela de medição, a decisão e as consequências

## Comments
