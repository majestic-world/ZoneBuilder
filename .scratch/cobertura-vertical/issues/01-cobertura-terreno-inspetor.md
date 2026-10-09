# 01: Cobertura do terreno no inspetor

**What to build:** com um shape selecionado, o inspetor mostra, além da medida em XY de hoje, a cobertura do shape sobre o terreno: o chão mais baixo e o mais alto sob o contorno (com o `x y z` de cada um), a folga do piso (`chãoMin − zmin`) e a do topo (`zmax − chãoMax`), e as áreas de chão dentro da faixa, acima do topo, abaixo do piso e sem chão (quad invisível, tile não carregado). Nesta fatia, chão é só o terreno. A medição é exata: recorta os triângulos do terreno pelo contorno, sem raios, então um pico entre os vértices conta (spec D1). Ela roda em segundo plano, com uma chave que muda quando o contorno, os tiles carregados ou a visibilidade dos meshes mudam, e o inspetor mostra o resultado anterior marcado como "medindo…" até o novo chegar, nunca números de outro contorno (spec D2). A medição separa o perfil do chão, que depende só do contorno e da cena, da classificação pela faixa, que é barata (spec D2, D3). Tudo em coordenadas do servidor (ADR 0003).

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] Teste do seam com chão sintético: uma pirâmide dentro de um quadrado dá o ápice como chão mais alto, mesmo com o ápice longe de qualquer vértice do contorno
- [x] Teste do seam: uma rampa sob um contorno em L (côncavo) dá a área acima do topo igual à fórmula fechada
- [x] Teste do seam: um quad invisível vai para "sem chão", e a cobertura não conta como 100%
- [x] Teste do seam: dentro + acima + abaixo é igual à área total de chão medida
- [x] Teste com o cliente real (pulado sem `ZB_CLIENT`): em 22_22, para 1000 colunas sorteadas sobre o terreno, o maior Z dos triângulos de chão da coluna é igual ao Z de um `Pick` vertical vindo de cima, com tolerância de 0,01
- [x] No app, o retângulo da captura sobre o morro mostra a folga do topo negativa e a área acima do topo maior que 0
- [x] Mover um vértice de um shape do tamanho de um tile mostra "medindo…" e depois o número novo, sem travar o viewport

## Comments

Implementado na branch `cv/01-cobertura-terreno-inspetor` e integrado em `cobertura-vertical`. Testes do seam em `internal/coverage` (pirâmide, rampa sob L côncavo, quad invisível, soma das áreas) e `TestFloorTopMatchesTheVerticalPick` no cliente real (1000 colunas em 22_22, ±0,01). Smoke: na captura, folga do topo −815 e acima do topo 83,8%; arrastar um vértice de shape do tamanho de um tile mostra "medindo…" e depois o número novo (pior quadro 21 ms). Perfil de um tile: 25 ms só com terreno.

Evidências: `C:/Workspace/zone-builder-notes/cobertura-vertical/01-evidence/`.
