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
Camadas de BSP ou de mesh que não cruzam a faixa Z do shape nem ficam a até 1024 unidades dela. Ficam fora dos extremos, das folgas, da cobertura e dos avisos, e não puxam a faixa nos ajustes. O terreno nunca é outra camada.

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
