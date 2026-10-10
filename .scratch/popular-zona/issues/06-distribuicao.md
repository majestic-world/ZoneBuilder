# 06: Distribuição no chão livre

**What to build:** o módulo puro `placement` (sem GL e sem Gio) distribui os pontos de uma área: `Distribute(g Geometry, r Request) Result` com o algoritmo da spec D3 (grade de 16, célula a pelo menos 1 raio da borda, chão de terreno ou BSP com `n.z ≥ 0,65` descendo do `zmax` ao `zmin` e nunca de mesh, fora d'água, obstáculo recortado na fatia `[chão − 16, chão + altura]` a menos de raio + afastamento em XY com descarte por caixa expandida, best-candidate de Mitchell k = 20 com PCG de `math/rand/v2`, distância mínima de 2 raios, posição dentro da célula e heading em 1..65535 do mesmo gerador). `Result` traz os pontos, quantos couberam, a área de chão livre, o espaçamento médio e o menor espaçamento. A ordem de iteração não depende de mapas Go nem da ordem de carga dos tiles. `scene.World` implementa `Geometry` com um método novo, irmão de `Floor`, que entrega todos os triângulos de uma caixa em coordenadas do servidor, com a superfície de cada um, e os volumes de água da caixa, respeitando as meshes ocultas como `Floor`.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] Teste do seam: campo plano sem obstáculos com N pequeno gera N pontos dentro do contorno, a pelo menos 1 raio da borda, com todos os pares a pelo menos 2 raios
- [ ] Teste do seam: mesma entrada e mesma semente dão pontos iguais; outra semente dá outros
- [ ] Teste do seam: caixa de mesh no meio do campo deixa livre só o que está a mais de raio + afastamento dela; mesh logo fora do contorno bloqueia a faixa de dentro
- [ ] Teste do seam: copa acima da altura do monstro não bloqueia; tronco que corta a fatia bloqueia
- [ ] Teste do seam: chão de mesh nunca recebe ponto, nem chão de terreno com `n.z < 0,65`
- [ ] Teste do seam: volume de água sobre metade do campo deixa essa metade sem pontos
- [ ] Teste do seam: ponte de BSP sobre o terreno, com a faixa no terreno põe os pontos no terreno, e com a faixa na ponte, na ponte
- [ ] Teste do seam: área pequena demais devolve K < N pontos, sem violar o espaçamento e sem laço infinito
- [ ] Teste com cliente real (pulado sem `ZB_CLIENT`): o tile do teste de cena, com uma área conhecida, gera pontos que nenhum triângulo de mesh do tile viola pela regra de obstáculo
