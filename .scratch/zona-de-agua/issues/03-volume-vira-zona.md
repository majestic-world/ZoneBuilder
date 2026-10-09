# 03: Volume vira zona de água, e ADR 0005

**What to build:** o pacote `internal/water` (novo, sem GL e sem Gio) transforma volumes de água em planos de zona: `Compile(volumes, existing) []Plan`. Os volumes são agrupados pelo topo no Z do servidor, com 1 zona por topo, porque o servidor usa o maior `zmax` da zona como superfície. Cada volume vira 1 polígono: a envoltória convexa XY dos vértices das faces, sem pontos colineares, arredondada, com o próprio `zmin = round(fundo) + ServerZOffset` e `zmax = round(topo) + ServerZOffset`. O tipo é `water`, sem parâmetros, e o nome é `[X_Y_<volume de menor nome>]`. Um nome que já existe no projeto não gera comandos: o plano devolve a zona existente. Os avisos de D7 (aproximada, água sobreposta, fora do tile) saem no plano. A constante `ServerZOffset` segue a decisão aprovada da spec D6, e o ADR 0005 registra a medição do datapack, a contradição com o ADR 0003 e a consequência visível: a zona compilada aparece 62 abaixo da superfície no viewport com −30.

**Blocked by:** 01 (Volumes de água na cena)

**Status:** resolved

- [x] Teste com o cliente real (pulado sem `ZB_CLIENT`): os 11 volumes de topo −3780 de 22_24 dão 1 zona com 11 polígonos iguais, ponto a ponto e em `zmin`/`zmax`, aos shapes `[22_24_water1…9]`, `[22_24_water12]` e `[22_24_water13]` do datapack, com os valores copiados no teste
- [x] Teste com o cliente real: `WaterVolume26` e `WaterVolume27` de 22_24 (encostados, topos −3746 e −5529) dão 2 zonas
- [x] Teste do seam: 2 topos dão 2 zonas, e cada polígono mantém o próprio `zmin`
- [x] Teste do seam: hexaedro com parede inclinada vira pegada convexa sem pontos colineares e com o aviso "aproximada"
- [x] Teste do seam: um volume que cruza em XY e Z outro volume vivo, de topo diferente e fora da seleção, gera o aviso de água sobreposta; encostar sem cruzar não gera
- [x] Teste do seam: com o nome já no projeto, o plano não tem comandos e aponta para a zona existente
- [x] Teste do seam: a zona gerada não tem nenhum problema de `Document.Problems()` (os comandos aplicados num documento vazio compilam sem `BlockedError`)
- [x] `docs/adr/0005-...md` escrito com a tabela de medição, a decisão e as consequências

## Comments

Implementado na branch `za/03-volume-vira-zona` e integrado em `zona-de-agua`. Desvio de D5: a assinatura é `water.Compile(selected, live []scene.WaterVolume, doc *zone.Document) []Plan`, porque o aviso de sobreposição precisa dos volumes vivos fora da seleção e o plano devolve o ID da zona existente ou reserva um novo. Testes: `TestFloranLakeReproducesTheDatapack`, `TestFloranFountainGivesOneZonePerTop` (zmax −3776 e −5559), `TestTwoTopsMakeTwoZonesAndEachPolygonKeepsItsFloor`, `TestSlantedWallBecomesConvexFootprintAndApproximate`, `TestOverlapWarnsOnlyWhenWaterCrosses`, `TestExistingNameCreatesNothingAndReusesTheZone`, `TestCompiledZonesHaveNoProblem`. Uma verificação de mutação mostrou que cada mutação reprova ao menos 1 teste. ADR: `docs/adr/0005-agua-30-abaixo-do-volume.md`, com o −30 pendente da medição em jogo (ticket 05).
