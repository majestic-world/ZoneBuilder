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
