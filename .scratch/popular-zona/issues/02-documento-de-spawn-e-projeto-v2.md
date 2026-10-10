# 02: Documento de spawn e projeto versão 2

**What to build:** o módulo `spawn` existe no molde de `zone.Document` (spec D2): `Area` (ID, nome, contorno XY sempre polígono, `ZMin`, `ZMax`, id do NPC, quantidade, respawn 60, `respawn_rand` 0, raio, afastamento 32, semente, pontos, impressão das entradas da última geração, oculta) e `Point` (`X, Y, Z, Heading` inteiros, coordenadas do servidor). Mutação só por `Apply(Command)` com histórico por snapshot: criar, apagar e duplicar área; editar vértices e mover (os comandos de vértice de `zone`, com outro alvo); faixa e parâmetros; `SetPoints` (pontos + semente + impressão + avisos, num único passo de desfazer); mover, apagar e adicionar ponto. A área fica com pontos desatualizados quando a impressão atual (contorno, faixa, quantidade, raio, afastamento, semente) difere da gravada; mexer em ponto não muda a impressão. `Problems()` em cache com todos os problemas que bloqueiam da spec D2 e os avisos gravados na geração (cabem K de N, nenhuma célula livre), que não bloqueiam. `MarshalJSON`/`UnmarshalJSON` com validação. `project.Project` ganha `Spawns`, a versão passa a 2, um arquivo da versão 1 abre com `Spawns` vazio, e versão maior que a atual continua recusada (spec D5). A compilação (`Compile`) é do ticket 09.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] Teste do seam: mudar contorno, faixa, quantidade, raio ou afastamento depois de `SetPoints` marca a área como desatualizada e gera o problema que bloqueia; mover, apagar ou adicionar ponto não marca
- [ ] Teste do seam: `respawn_rand > respawn`, id 0, quantidade 0, nome vazio, nome repetido, área sem pontos, contorno com menos de 3 vértices ou auto-interseção, `zmin > zmax` e ponto fora dos limites do mundo bloqueiam; os avisos "cabem K de N" e "nenhuma célula livre" não bloqueiam
- [ ] Teste do seam: 1 `Undo` desfaz uma geração inteira (`SetPoints`)
- [ ] Teste de `project`: um projeto com 2 áreas com pontos, salvo e reaberto, devolve áreas e pontos idênticos; um arquivo da versão 1 abre sem áreas e com as zonas intactas; o teste de recusa de versão maior que a atual continua passando
