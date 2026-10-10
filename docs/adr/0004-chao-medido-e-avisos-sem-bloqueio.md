# Chão medido sobre os triângulos, e avisos de cobertura que não bloqueiam a compilação

Um shape é um prisma: o servidor só considera um personagem dentro da zona quando o `x y` está no contorno e o `z` está entre `zmin` e `zmax`. O app sugeria a faixa só pelo Z dos vértices e não mostrava o chão entre eles, então um morro no meio de um retângulo furava o topo sem que ninguém visse. Para medir a cobertura, o app precisa de uma definição de chão e de uma política para quando o chão sai da faixa.

## Decisão 1: o que é chão, e como é medido

**Chão** é a superfície onde um personagem pode ficar:

- todo quad visível do terreno (`QuadVisibility`), com 2 triângulos na diagonal de `EdgeTurn`, a mesma regra do desenho e do picking;
- mais as faces de BSP e de static mesh voltadas para cima, com `normal.z ≥ 0,5`;
- as meshes só entram com o botão **Static meshes** ligado (`World.HideMeshes`): o que se vê é o que se mede. Num shape em Giran (22_22), o máximo de camadas de chão é 6 com as meshes visíveis e 2 com elas ocultas.

Tudo sai em coordenadas do servidor (`scene.ToServer`, [ADR 0003](0003-z-do-servidor-32-acima-do-cliente.md)).

A medição é **exata sobre os triângulos**, sem raios. Dentro de cada triângulo a altura é afim em `x y`, então, recortando os triângulos pelo contorno (só nas células do perímetro; um triângulo com os 3 vértices dentro e nenhuma aresta cruzando o contorno entra inteiro, o caso comum; os outros passam por Sutherland–Hodgman, com o contorno, que pode ser côncavo, como polígono **sujeito** e o triângulo, convexo, como **janela**):

- o chão mais alto e o mais baixo caem num vértice de uma peça recortada, sem depender de resolução;
- a área acima de `zmax` ou abaixo de `zmin` é a peça cortada por um semiplano, também exata;
- a linha do chão ao longo de cada aresta do contorno é a polilinha dos cruzamentos com as arestas dos triângulos, só no chão que a regra das camadas conta para a faixa. Ela sai da CPU (`Report.Edges`), não de uma comparação de profundidade no shader: o depth do viewport é um renderbuffer `DEPTH_COMPONENT32F` (`internal/render/target.go`), que o shader não pode amostrar. As linhas de nível no chão (z = `zmax` ou `zmin`) são do shader, com `|vPos.z − z| / fwidth(vPos.z)` e os uniforms `uShapeZ`.

O perfil (caro, depende do contorno e da cena) é calculado fora do loop, com orçamento de 100 ms; a classificação pela faixa (barata) roda no loop, no mesmo frame do arrasto, com orçamento de 2 ms. Medido com `-fps`:

- perfil de 1 tile (cerca de 256×256 amostras, uns 130 mil triângulos de terreno): 25 ms só com terreno, cerca de 98 ms com meshes (22_22 inteiro);
- classificação no 22_22 inteiro com meshes: 1,2 a 1,4 ms em média;
- arrastar um vértice de um shape do tile inteiro: pior frame de 21 ms; arrastar a seta Z de uma zona do tile inteiro: cerca de 65 fps, igual a uma zona pequena; a régua roda a 81–85 fps no arrasto.

p99 da classificação, em ms, com timer QPC ao arrastar a seta Z (320 a 960 amostras por caso), antes e depois do [ADR 0006](0006-terreno-como-camada.md):

| Caso | Antes | Depois |
| --- | --- | --- |
| Zona do relato no 23_18 | 0,088 | 0,092 |
| Tile inteiro do 23_18 | 1,760 | 1,822 |
| Morro no 22_22 | 0,022 | 0,022 |
| Tile inteiro do 22_22 | 1,587 | 1,600 |

Outra sessão mediu o tile inteiro do 23_18 em 2,04 ms, antes e depois: a variação entre sessões cruza os 2 ms, então o tile inteiro do 23_18 está no limite do orçamento. Na mesma sessão: polígono da zona do relato no 22_22 0,018 ms, tile inteiro do 22_22 1,84 ms, polígono da torre do 23_18 0,16 ms.

Uma tabela de área acumulada por unidade de Z (entre o chão mínimo e o máximo), que faria da classificação 2 consultas, estava prevista só se a classificação passasse de 2 ms. Não passou na média, então a tabela **não foi feita**; o histograma da régua sai direto do perfil (`Profile.Histogram()`). Se o perfil ficar lento em zonas grandes de cidade, a mitigação prevista é filtrar os `pickables` pela caixa antes de percorrer os triângulos deles.

**Prévia = criação.** Para retângulo e círculo, a prévia (`placed` em `cmd/zonebuilder/shapetool.go`) é exatamente o que entra no documento. A prévia pede o perfil do contorno sob o cursor de forma assíncrona, com a mesma chave de cache; até ele chegar, mostra a faixa pelos vértices marcada "medindo…". O clique final mede de forma **síncrona** (no máximo cerca de 100 ms, uma vez) se o perfil ainda não chegou. Enquanto um perfil é calculado, a interface mostra o anterior marcado "medindo…", nunca números de outro contorno.

**Exclusões:** uma peça cujo centróide cai num shape banido da zona, e na faixa Z dele, conta como excluída. O erro fica limitado a 1 célula ao longo do contorno da exclusão. O chão excluído não define os extremos, as folgas, os pinos nem os avisos, e não puxa a faixa nos ajustes. **Descartada:** subtrair as exclusões de forma exata, que exigiria diferença de polígonos (contorno menos shapes banidos) e não vale o custo.

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

**Pendente:** a verificação em jogo não foi feita. Desde o [ADR 0008](0008-sem-avisos-de-chao.md) não há mais limiares de aviso (`MinClearance`, `StrayDepth`, `StrayShare`) a confirmar; ela só confirma o ADR 0003 e o significado das folgas.

Ponto de reprodução: `C:/Workspace/zone-builder-notes/cobertura-vertical/captura.zbproj` (mapa 22_22). O pino do topo (chão mais alto) fica em **76288 162688 −1240** (servidor); a faixa do shape ali é **−2754 … −2055**, então a folga do topo é **−815**, e o app mostra 83,8% da área acima do topo.

1. Com um GM parado nesse pino, comparar o `//pos` com Z −1240. Uma diferença acima de 16 reabre o `ServerZOffset` do ADR 0003.
2. Fazer uma zona sobre um morro com folga do topo entre 0 e 32, compilar, reiniciar o servidor e ver se o personagem no pico conta como dentro.
3. Anotar se algum mesh sem colisão aparece como chão que o servidor não tem.

## Consequences

- A cobertura é a do cliente, não a da geodata: o app não lê o datapack, nem para medir o chão nem para decidir camadas. Um mesh sem colisão pode aparecer como chão que o servidor não tem.
- A folga herda o erro empírico do `ServerZOffset` (ADR 0003); o `MinClearance` cobre o p95 medido.
- Shapes que se sobrepõem contam a área de chão em dobro no resumo da zona, que diz "soma dos shapes".
- O shader desenha a pegada (só no chão visto de cima, [ADR 0010](0010-prisma-oco-em-aneis.md)) de até 8 shapes e 128 pontos (`groundMaxShapes`, `groundMaxPoints`); acima disso o inspetor avisa "pegada parcial". Os números não têm esse limite.
