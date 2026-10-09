# 01: Cobertura do terreno no inspetor

**What to build:** com um shape selecionado, o inspetor mostra, além da medida em XY de hoje, a cobertura do shape sobre o terreno: o chão mais baixo e o mais alto sob o contorno (com o `x y z` de cada um), a folga do piso (`chãoMin − zmin`) e a do topo (`zmax − chãoMax`), e as áreas de chão dentro da faixa, acima do topo, abaixo do piso e sem chão (quad invisível, tile não carregado). Nesta fatia, chão é só o terreno. A medição é exata: recorta os triângulos do terreno pelo contorno, sem raios, então um pico entre os vértices conta (spec D1). Ela roda em segundo plano, com uma chave que muda quando o contorno, os tiles carregados ou a visibilidade dos meshes mudam, e o inspetor mostra o resultado anterior marcado como "medindo…" até o novo chegar, nunca números de outro contorno (spec D2). A medição separa o perfil do chão, que depende só do contorno e da cena, da classificação pela faixa, que é barata (spec D2, D3). Tudo em coordenadas do servidor (ADR 0003).

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] Teste do seam com chão sintético: uma pirâmide dentro de um quadrado dá o ápice como chão mais alto, mesmo com o ápice longe de qualquer vértice do contorno
- [ ] Teste do seam: uma rampa sob um contorno em L (côncavo) dá a área acima do topo igual à fórmula fechada
- [ ] Teste do seam: um quad invisível vai para "sem chão", e a cobertura não conta como 100%
- [ ] Teste do seam: dentro + acima + abaixo é igual à área total de chão medida
- [ ] Teste com o cliente real (pulado sem `ZB_CLIENT`): em 22_22, para 1000 colunas sorteadas sobre o terreno, o maior Z dos triângulos de chão da coluna é igual ao Z de um `Pick` vertical vindo de cima, com tolerância de 0,01
- [ ] No app, o retângulo da captura sobre o morro mostra a folga do topo negativa e a área acima do topo maior que 0
- [ ] Mover um vértice de um shape do tamanho de um tile mostra "medindo…" e depois o número novo, sem travar o viewport
