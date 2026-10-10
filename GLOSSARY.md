# Glossário

Termos do domínio do Zone Builder. Use estes nomes em código, issues, testes e docs.

## Cobertura vertical

**Chão**:
Superfície onde um personagem pode ficar: todo quad visível do terreno, mais as faces de BSP e de static mesh voltadas para cima (`normal.z ≥ 0,5`). Meshes só contam com o botão Static meshes ligado. Medido em coordenadas do servidor (ADR 0003, ADR 0004).
_Evite_: terreno (é só uma parte do chão), piso (é o `zmin` da zona), solo.

**Camada**:
Cada superfície de chão empilhada numa mesma coluna `x y`: terreno, piso de prédio, ponte.
_Evite_: andar, nível.

**Outras camadas**:
Camadas que não cruzam a faixa Z do shape nem ficam a até 1024 unidades dela. Ficam fora dos extremos, das folgas, da cobertura e dos avisos, e não puxam a faixa nos ajustes. Inclui o terreno, julgado como um bloco só (todo o terreno sob o contorno): ele é outra camada quando nenhuma parte dele alcança a faixa e alguma peça de BSP ou mesh alcança; se nenhuma alcança, o terreno conta (ADR 0006).

**Origem da faixa nova**:
De onde um polígono, retângulo ou círculo novo tira `zmin zmax`: **chão da área** (padrão, pelo perfil do chão sob o contorno) ou **pontos clicados** (`menorZ − folga … maiorZ + folga` dos vértices, sem medir o chão). O tile inteiro sempre usa o chão da área. Escolhida no seletor "Faixa nova" do inspetor e guardada só na sessão.
_Evite_: modo de sugestão.

**Anel**:
Linha horizontal desenhada na parede do prisma de um shape a cada passo de Z, contado a partir do `zmin` dele. O passo-base é 64 e dobra até os anéis ficarem a pelo menos 6 px um do outro na tela. O prisma não tem tampas nem preenchimento: são só anéis, arestas e contornos.
_Evite_: listra, hachura (é a marca do chão fora da faixa).

**Excluída**:
Área de chão cujo centróide cai dentro de uma exclusão (shape banido) da zona e dentro da faixa Z dela. Não conta como falha nem define o pior ponto.
_Evite_: banida (é o shape, não o chão).

**Perfil do chão** (de um shape):
O chão recortado pelo contorno do shape: peças planas, extremos e o chão ao longo de cada aresta. Depende só do contorno e da cena, não da faixa Z.
_Evite_: medição, amostragem.

**Folga do piso**:
`chãoMin − zmin`. Negativa quando parte do chão está abaixo do piso.

**Folga do topo**:
`zmax − chãoMax`. Negativa quando o chão fura o topo.
_Evite_: margem (é o valor somado ao chão nos ajustes, padrão 256).

**Cobertura**:
Fração da área de chão sob o contorno que fica dentro da faixa Z.

**Sem chão**:
Área do contorno onde não há chão (quad invisível, tile não carregado, fora do mapa). Não é medida e aparece como tal, nunca como coberta.

**Pior ponto**:
O `x y z` do chão com a menor folga, separado para o piso (chão mais baixo) e para o topo (chão mais alto). É onde ficam os pinos e para onde os avisos levam a câmera.

## Água

**Volume de água**:
Um ator `WaterVolume` vivo do mapa (em `Level.Actors`, sem `bDeleteMe`): brush convexo lido de todas as faces BSP do Model dele, em coordenadas do cliente. No código, `scene.WaterVolume`; identificado por tile e export.
_Evite_: superfície de água (é o material desenhado no pass Water; 70 superfícies não têm volume nenhum), lago (não é termo do código).

**Topo**:
O maior Z das faces do volume, a superfície da água no cliente. Vira o `zmax` da zona com `water.ServerZOffset` (−30, ADR 0005, pendente da medição em jogo), nunca com o +32 de `scene.ToServer`. Volumes de topos diferentes viram zonas diferentes.
_Evite_: superfície, nível da água.

**Exata / aproximada**:
Exata: paredes verticais e topo e fundo horizontais, então o prisma do servidor é o próprio volume. Aproximada: parede ou topo inclinado, e o prisma cobre o volume com sobra; a compilação avisa.

## População

**Área de spawn**:
Polígono `x y` do modo população, com faixa Z, id do NPC, quantidade, respawn, `respawn_rand`, raio, afastamento, semente e pontos. Não é uma zona: não tem tipo nem exclusões (o servidor não suporta exclusão em spawn) e compila para o XML de spawn, não para o de zona. No código, `spawn.Area`.
_Evite_: zona de spawn, território.

**Ponto de spawn**:
Um monstro da área: `x y z heading` em coordenadas do servidor, com heading em 1..65535. Vira 1 `<spawn>` com 1 `<npc count="1" pos="x y z h">` (ADR 0007). Gerado pela distribuição ou movido, apagado e adicionado à mão.
_Evite_: pino (é o desenho do ponto no viewport).

**Raio**:
Raio de colisão do monstro. Afasta os pontos da borda da área e um ponto do outro (no mínimo 2 × raio). O padrão é 9, o raio medido na mesh do monstro de prévia (8,75) arredondado.

**Afastamento**:
Distância a mais, além do raio, que o ponto guarda das static meshes e das paredes. O padrão é 32.
_Evite_: folga (é a do piso e do topo), margem (é a dos ajustes de faixa).

**Célula livre**:
Célula de 16 unidades com o centro dentro da área e a pelo menos 1 raio da borda, com chão de terreno ou BSP (`n.z ≥ 0,65`) dentro da faixa Z, fora d'água e sem mesh nem parede a até raio + afastamento na fatia da altura do monstro. Todas as meshes contam, mesmo com o botão Static meshes desligado.

**Semente**:
Número que determina o sorteio dos pontos e dos headings. A mesma semente, com as mesmas entradas, dá os mesmos pontos. **Gerar** usa a semente da área; **Regerar** troca a semente e descarta os ajustes manuais (1 passo de desfazer).

**Pontos desatualizados**:
O contorno, a faixa Z, a quantidade, o raio, o afastamento ou a semente mudaram depois da última geração. Bloqueia a compilação até gerar de novo. Nome, NPC, respawn e ajustes manuais dos pontos não desatualizam.
_Evite_: pontos velhos, sujos.

**Monstro de prévia**:
O modelo embutido que a prévia desenha em todo ponto, seja qual for o id: o `death_knight_wizard_m00`, na animação `Wait`, virado pelo heading. Tem 66 de altura.
_Evite_: modelo do NPC (o app não sabe qual é).

**Modo jogo**:
O port do Play Map do UE2-Studio, aberto pelo botão **Jogar** nos 2 modos: o humano embutido numa cápsula com gravidade e colisão, em terceira ou primeira pessoa. Não é um terceiro modo: fica na frente do modo ativo, que não recebe entrada, e Esc volta à câmera de edição.
_Evite_: modo de teste, play.
