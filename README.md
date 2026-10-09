# Zone Builder

Editor desktop de zonas para servidores de Lineage II. O Zone Builder abre os mapas do cliente em 3D, deixa marcar as zonas com o mouse direto sobre o terreno e gera o XML no formato que o servidor Java carrega de `data/zone/`.

## O que ele faz

- Abre os tiles do cliente (`Maps/X_Y.unr`) com terreno, BSP e static meshes texturizados, sozinhos ou com os vizinhos (até 3×3).
- Cria zonas por polígono, retângulo, círculo ou o tile inteiro, inclusive exclusões (`banned_polygon`), e adiciona restart points e PK restart points.
- Cada clique cai no ponto visível sob o cursor, em coordenadas do servidor.
- Edita vértices, shapes, faixa Z, altura da zona, tipo e parâmetros `<set>`, com desfazer e refazer.
- Com o botão **Chão** ligado, desenha a grade das células do terreno, com uma linha forte a cada 8 células. O comprimento de cada aresta aparece no viewport, e o tamanho, a área e o perímetro do shape aparecem no inspetor.
- Mostra, sem precisar ligar nada, se a faixa Z da zona selecionada cobre o chão dentro do contorno: cores no chão, a linha do chão nas paredes, pinos no pior ponto, números no inspetor, uma régua na janela de altura e avisos no painel de problemas. Veja [Cobertura vertical](#cobertura-vertical).
- Mostra os problemas de cada zona enquanto você edita, como polígono que se cruza, nome repetido ou faixa Z invertida, e bloqueia a compilação até corrigir. Os avisos de chão aparecem na mesma lista, mas não bloqueiam.
- Compila as zonas selecionadas em um arquivo por tipo (`zonebuilder_<tipo>.xml`) e mostra o XML numa janela com botão de copiar.
- Guarda o trabalho em projetos `.zbproj`, inclusive zonas ainda incompletas.

## Requisitos

- Windows 64 bits.
- Go 1.27 ou mais novo, para compilar.
- PowerShell 7 (`pwsh`) e `make`.
- Um cliente de Lineage II com a pasta `Maps`.

## Compilar e executar

```sh
make build   # gera "bin/Zone Builder.exe" com as DLLs do ANGLE ao lado
make run     # compila e abre o app
make run ARGS="-project giran.zbproj"
```

A versão exibida no título da janela vem de `APP_VERSION`, no arquivo `.env`.

Opções de linha de comando:

| Opção | Uso |
|---|---|
| `-client` | pasta do cliente (a que contém `Maps`) |
| `-tile` | tile que o campo traz preenchido, por exemplo `22_22` |
| `-project` | projeto `.zbproj` aberto ao iniciar |
| `-camera` | pose inicial da câmera, `x,y,z,yaw,pitch` |
| `-fps` | mede a taxa de quadros e registra no log |

Sem `-client`, o app usa a última pasta salva na configuração do usuário (`%AppData%\ZoneBuilder\config.json`), depois a variável de ambiente `ZB_CLIENT`.

## Como usar

1. Informe a pasta do cliente e o tile, e clique em **Abrir**.
2. Em **Nova zona**, digite o nome, escolha o tipo e clique em **Criar zona e desenhar**.
3. Clique sobre o mapa para marcar os vértices; Enter ou um clique no primeiro vértice fecha o polígono.
4. Ajuste altura, tipo e parâmetros no inspetor à direita e na janela **Altura da zona**.
5. Clique em **Compilar XML**, copie cada arquivo da janela e cole em `data/zone/` do servidor com o nome indicado.

Controles do viewport:

| Tecla ou gesto | Ação |
|---|---|
| W, A, S, D | andar e mover para os lados |
| E, Q | subir e descer |
| Shift | velocidade alta |
| arrastar | olhar em volta |
| roda do mouse | avançar e recuar |
| PageUp, PageDown | subir e descer a zona selecionada |
| Enter | fechar o polígono em desenho |
| Delete | apagar o vértice selecionado |
| Esc | soltar a ferramenta |
| Ctrl+Z, Ctrl+Y | desfazer e refazer |

## Cobertura vertical

O servidor só considera um personagem dentro da zona quando o `x y` dele está no contorno **e** o `z` está entre `zmin` e `zmax`. Chão fora dessa faixa é chão fora da zona: o XML compila, mas a zona não vale ali em jogo. O app mede o chão sob cada shape e mostra o resultado na zona selecionada, com o botão **Chão** ligado ou não. Os termos (chão, camada, folga, cobertura, pior ponto) estão definidos em [`GLOSSARY.md`](GLOSSARY.md), e as decisões, no [ADR 0004](docs/adr/0004-chao-medido-e-avisos-sem-bloqueio.md).

### No viewport

| O que aparece | Significado |
|---|---|
| Chão tingido com a cor da zona | chão dentro do contorno e dentro da faixa Z: coberto |
| Hachura laranja (diagonal num sentido) | chão acima do topo (`zmax`): um morro ou piso que fura o topo |
| Hachura azul (diagonal no outro sentido) | chão abaixo do piso (`zmin`): um vale ou piso que passa por baixo |
| Buraco, sem tinta | chão dentro de uma exclusão |
| Linha laranja no chão | onde o chão cruza o topo (`z = zmax`): fronteira exata da parte acima |
| Linha azul no chão | onde o chão cruza o piso (`z = zmin`): fronteira exata da parte abaixo |
| Linha na cor da aresta, em cada parede do prisma | o chão ao longo daquela aresta, do shape selecionado; some enquanto o shape é medido de novo |
| Aresta ou linha sólida | parte visível |
| Aresta ou linha tracejada, faces com listras fracas | parte do prisma enterrada ou atrás da cena, vista através do terreno |
| Pino com rótulo `topo +412` | chão mais alto do shape e a folga do topo nele |
| Pino com rótulo `piso −96` | chão mais baixo do shape e a folga do piso nele |
| Rótulo vermelho | folga negativa: o chão sai da faixa naquele ponto |

Clicar no rótulo de um pino leva a câmera ao ponto. O shader desenha a pegada de até 8 shapes e 128 pontos; acima disso, o inspetor diz "Pegada parcial: N shapes fora do desenho". Os números não têm esse limite.

### Números

O inspetor mostra, para o shape selecionado:

- **Chão mais baixo** e **Chão mais alto**, com `x y z`;
- **Folga do piso** (`chãoMin − zmin`) e **folga do topo** (`zmax − chãoMax`). Negativa quer dizer que fura e aparece como `−96 (fura)`;
- **Chão em N camadas**: o maior número de superfícies empilhadas numa coluna (terreno, piso de prédio, ponte);
- a área de chão, em unidades² e em % do chão sob o contorno: **Dentro da faixa**, **Acima do topo**, **Abaixo do piso**, **Excluída** (dentro de uma exclusão da zona e na faixa Z dela), **Em outras camadas** e **Sem chão** (quad invisível, tile não carregado, fora do mapa);
- **Outras camadas**, com o Z de cada uma: pisos de BSP ou mesh longe demais da faixa para contar (veja a regra das camadas abaixo).

Enquanto um contorno é medido, os números anteriores ficam com a marca "medindo…". Arrastar a seta Z ou mudar a faixa atualiza números, cores e pinos no mesmo frame.

### Régua da janela de altura

A janela **Altura da zona** traz uma régua vertical, que não depende do ângulo da câmera:

- a barra na cor da zona é a faixa Z, do menor `zmin` ao maior `zmax` dos shapes;
- ao lado, o histograma da área de chão por Z, com as mesmas cores do chão: tinta dentro, laranja acima, azul abaixo;
- as marcas `piso` e `topo` ficam no chão mais baixo e no mais alto, com a folga escrita, em vermelho quando negativa.

Embaixo, 3 linhas somam os shapes da zona (shapes que se sobrepõem contam a área em dobro):

```
Chão sob a zona: 1.204 … 1.980
Folga do piso: 248 · Folga do topo: −96 (fura)
Cobertura 97,3% · acima 2,7% · abaixo 0% · sem chão 0%
```

### Avisos

O painel de problemas lista os avisos de chão depois dos erros, com outro ícone. **Eles não bloqueiam a compilação**: há casos legítimos de chão fora da faixa, como uma zona só no andar de cima ou uma área em tiles que não foram abertos. Clicar num aviso seleciona a zona e o shape e leva a câmera ao pior ponto.

| Aviso | Quando |
|---|---|
| Chão acima do topo | pelo menos 1% do chão acima do topo, ou folga do topo menor que −16 |
| Chão abaixo do piso | pelo menos 1% do chão abaixo do piso, ou folga do piso menor que −16 |
| Folga apertada | folga do piso ou do topo entre 0 e 32 |
| Área sem chão medido | parte do contorno sem chão, por exemplo num tile que não está aberto |

O chão excluído e o das outras camadas não geram aviso. Sem um tile aberto não há avisos.

### Ajustar ao chão

- **Recalcular pelo chão**, no inspetor, põe `zmin` no chão mais baixo menos a folga Z e `zmax` no chão mais alto mais a folga (padrão 256), olhando o chão da área inteira, não só os vértices.
- **Piso ao chão** e **Topo ao chão**, na janela de altura, mexem só num lado, em todos os shapes incluídos da zona. As exclusões ficam com a faixa delas. Shapes sem chão medido, ou em que o lado novo passaria do outro, ficam como estão e são contados na mensagem.
- Cada botão é 1 passo de desfazer.
- Ao criar um retângulo, círculo, polígono ou tile inteiro, a faixa sugerida sai do chão da área da mesma forma. A mensagem diz de onde ela veio: "pelo chão da área", ou "pelos vértices" quando parte da área está fora dos tiles carregados ou não há chão medido.

**Regra das camadas.** O terreno sempre conta. Um piso de BSP ou de static mesh só puxa a faixa quando cruza a faixa atual ou fica a até 1024 unidades dela; os outros aparecem como "outras camadas". Assim o telhado de uma torre ou uma caverna muito abaixo não esticam a faixa de uma zona de rua. O chão excluído também não puxa. Com o botão **Static meshes** desligado, meshes não entram na medição.

A regra é aplicada uma vez, a partir da faixa de antes do clique. **Consequência aceita:** logo depois de um ajuste, uma camada que estava longe da faixa antiga mas fica perto da nova aparece como acima ou abaixo da faixa, com aviso; apertar o botão de novo a puxa para dentro. Repita só se essa camada deve mesmo fazer parte da zona.

## Licenças de terceiros

- ANGLE (`third_party/angle`): BSD.
- Ícones Lucide (`internal/ui/icon/lucide`): ISC.
- Fonte Inter (`internal/ui/fonts`): SIL Open Font License.
