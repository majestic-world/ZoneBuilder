# Chão medido sobre os triângulos, e avisos de cobertura que não bloqueiam a compilação

Um shape é um prisma: o servidor só considera um personagem dentro da zona quando o `x y` está no contorno e o `z` está entre `zmin` e `zmax`. O app sugeria a faixa só pelo Z dos vértices e não mostrava o chão entre eles, então um morro no meio de um retângulo furava o topo sem que ninguém visse. Para medir a cobertura, o app precisa de uma definição de chão e de uma política para quando o chão sai da faixa.

## Decisão 1: o que é chão, e como é medido

**Chão** é a superfície onde um personagem pode ficar:

- todo quad visível do terreno (`QuadVisibility`), com 2 triângulos na diagonal de `EdgeTurn`, a mesma regra do desenho e do picking;
- mais as faces de BSP e de static mesh voltadas para cima, com `normal.z ≥ 0,5`;
- as meshes só entram com o botão **Static meshes** ligado (`World.HideMeshes`): o que se vê é o que se mede.

Tudo sai em coordenadas do servidor (`scene.ToServer`, [ADR 0003](0003-z-do-servidor-32-acima-do-cliente.md)).

A medição é **exata sobre os triângulos**, sem raios. Dentro de cada triângulo a altura é afim em `x y`, então, recortando os triângulos pelo contorno (Sutherland–Hodgman, só nas células do perímetro):

- o chão mais alto e o mais baixo caem num vértice de uma peça recortada, sem depender de resolução;
- a área acima de `zmax` ou abaixo de `zmin` é a peça cortada por um semiplano, também exata;
- a linha do chão ao longo de cada aresta do contorno é a polilinha dos cruzamentos com as arestas dos triângulos, só no chão que a regra das camadas conta para a faixa.

O perfil (caro, depende do contorno e da cena) é calculado fora do loop; a classificação pela faixa (barata) roda no loop, no mesmo frame do arrasto. Medido no 22_22 inteiro com meshes: perfil em cerca de 98 ms fora do loop e classificação em 1,2 ms em média, p99 abaixo de 2 ms.

**Exclusões:** uma peça cujo centróide cai num shape banido da zona, e na faixa Z dele, conta como excluída. O erro fica limitado a 1 célula ao longo do contorno da exclusão. O chão excluído não define os extremos, as folgas, os pinos nem os avisos, e não puxa a faixa nos ajustes.

**Camadas:** substituído pelo [ADR 0006](0006-terreno-como-camada.md), que faz o terreno seguir a regra de alcance, julgado em bloco e com fallback. O texto original fica abaixo como histórico.

> O terreno sempre conta. Uma camada de BSP ou de mesh só conta quando cruza a faixa ou fica a até `GroundReach = 1024` dela; as outras vão para "outras camadas" e ficam fora dos extremos, das folgas, dos avisos e do histograma.

Continua valendo: nos ajustes ("Recalcular pelo chão", "Piso ao chão", "Topo ao chão") a regra é julgada **uma vez**, a partir da faixa de antes do clique. Iterar até um ponto fixo foi testado e descartado: no tile inteiro, a faixa subia de piso em piso até o telhado de uma torre (z 7105). A consequência aceita é que, logo depois de um ajuste, uma camada que ficou ao alcance da faixa nova aparece como acima ou abaixo dela, com aviso, e apertar o botão de novo a puxa.

**Descartada:** amostrar com raios verticais numa grade. Perde picos entre as amostras, custa um `Pick` (que percorre todos os pickables) por raio e deixa o erro dependendo do passo.

## Decisão 2: avisar, não bloquear

**Avisos:** substituído pelo [ADR 0008](0008-sem-avisos-de-chao.md), que remove os avisos de chão. O texto original fica abaixo como histórico; continua valendo que chão fora da faixa não bloqueia a compilação.

Chão fora da faixa vira **aviso** no painel de problemas, com ícone próprio. Ele não entra em `Document.Problems()` e não bloqueia a compilação. Há casos legítimos: uma zona só no andar de cima de um prédio, uma zona que deixa o topo de um morro de fora de propósito, uma área em tiles que não foram abertos. Clicar no aviso leva a câmera ao pior ponto.

| Aviso | Quando |
| --- | --- |
| Chão acima do topo | área acima do topo ≥ 1% do chão (`StrayShare`) ou folga do topo < −16 (`StrayDepth`) |
| Chão abaixo do piso | área abaixo do piso ≥ 1% ou folga do piso < −16 |
| Folga apertada | folga do piso ou do topo em `[0, MinClearance)`, com `MinClearance = 32` |
| Área sem chão medido | parte do contorno sem chão |

- **1% e 16** existem para não acusar lascas de 1 célula em penhascos.
- **`MinClearance = 32`:** o ADR 0003 mediu a geodata em média 32 acima do cliente, mas com p95 de −19,9 (até cerca de 12 acima do que o offset prevê) e quantização de 8. Uma folga menor que 32 pode cair fora da faixa no servidor mesmo aparecendo dentro no app. **[INFERENCE]**, a confirmar na verificação em jogo.

## Verificação em jogo

**Pendente:** depende do ticket 13 (`ready-for-human`), que ainda não foi feito. Os limiares acima são, por enquanto, os do spec. Quando o ticket 13 for feito, esta seção registra:

- o pior ponto que o app relatar num shape, o `//pos` do GM parado em cima dele e a diferença entre os 2 Z. Uma diferença acima de 16 reabre o `MinClearance` e o ADR 0003;
- uma zona com folga do topo entre 0 e 32 sobre um morro: se o servidor reconhece o personagem dentro dela;
- se algum mesh sem colisão aparece como chão que o servidor não tem.

## Consequences

- A cobertura é a do cliente, não a da geodata: o app não lê o datapack. Um mesh sem colisão pode aparecer como chão que o servidor não tem.
- A folga herda o erro empírico do `ServerZOffset` (ADR 0003); o `MinClearance` cobre o p95 medido.
- Shapes que se sobrepõem contam a área de chão em dobro no resumo da zona, que diz "soma dos shapes".
- O shader desenha a pegada de até 8 shapes e 128 pontos; acima disso o inspetor avisa "pegada parcial". Os números não têm esse limite.
