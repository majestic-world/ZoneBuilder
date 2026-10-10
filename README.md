# Zone Builder

Editor desktop de zonas e spawns para servidores de Lineage II. O Zone Builder abre os mapas do cliente em 3D, deixa marcar as zonas com o mouse direto sobre o terreno e gera o XML no formato que o servidor Java carrega de `data/zone/`. No modo **Popular zona**, distribui monstros pelo chão livre de uma área, mostra cada um na prévia e gera o XML de spawn de `data/spawn/`.

## O que ele faz

- Abre os tiles do cliente (`Maps/X_Y.unr`) com terreno, BSP e static meshes texturizados, sozinhos ou com os vizinhos (até 3×3).
- Cria zonas por polígono, retângulo, círculo ou o tile inteiro, inclusive exclusões (`banned_polygon`), e adiciona restart points e PK restart points.
- Cada clique cai no ponto visível sob o cursor, em coordenadas do servidor.
- Edita vértices, shapes, faixa Z, altura da zona, tipo e parâmetros `<set>`, com desfazer e refazer.
- Com o botão **Chão** ligado, desenha a grade das células do terreno, com uma linha forte a cada 8 células. O comprimento de cada aresta aparece no viewport, e o tamanho, a área e o perímetro do shape aparecem no inspetor.
- Mostra, sem precisar ligar nada, se a faixa Z da zona selecionada cobre o chão dentro do contorno: cores no chão, a linha do chão nas paredes, pinos no pior ponto, números no inspetor, uma régua na janela de altura e avisos no painel de problemas. Veja [Cobertura vertical](#cobertura-vertical).
- Gera a zona `water` direto do `WaterVolume` do cliente: clique na água, botão direito, **Compilar zona de água**. Veja [Zona de água](#zona-de-água).
- Mostra os problemas de cada zona enquanto você edita, como polígono que se cruza, nome repetido ou faixa Z invertida, e bloqueia a compilação até corrigir. Os avisos de chão aparecem na mesma lista, mas não bloqueiam.
- Compila as zonas selecionadas em um arquivo por tipo (`zonebuilder_<tipo>.xml`) e mostra o XML numa janela com botão de copiar.
- Guarda o trabalho em projetos `.zbproj`, inclusive zonas ainda incompletas.
- Popula áreas com monstros: distribui os pontos de spawn longe das static meshes, mostra o monstro de prévia em cada ponto e compila um `<spawn>` com `pos` por ponto. Veja [Popular zona](#popular-zona).
- Anda pelo mapa com um personagem, com gravidade e colisão, nos 2 modos. Veja [Modo jogo](#modo-jogo).

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
| `-mode` | abre direto num modo, sem a tela inicial: `zones` (Construir zonas) ou `populate` (Popular zona); sem a opção, o app abre na tela inicial, mesmo com `-project` |

Sem `-client`, o app usa a última pasta salva na configuração do usuário (`%AppData%\ZoneBuilder\config.json`), depois a variável de ambiente `ZB_CLIENT`.

## Traduções para desenvolvimento

Mensagens da interface ficam embutidas em `internal/locale/catalog/<área>/pt-BR.json` e `en.json`. Ao acrescentar uma área, crie **os dois arquivos**: cada chave estável começa com `<área>.`, e os dois idiomas precisam conter as mesmas chaves, os mesmos parâmetros nomeados (`{name}`) e, para mensagens com contagem, as variantes `one` e `other`. A forma `one` vale somente para 1; `other` vale para 0 e demais contagens. Use uma frase completa por variante, não fragmentos concatenados.

Apresente texto estático com `locale.Text`, texto parametrizado com `locale.Format` e contagem com `locale.Plural`. Números exibidos usam `locale.Number` ou `locale.Percent` antes da interpolação; coordenadas, valores digitados, projetos e XML permanecem literais. Para status que sobrevivem à troca de idioma, retenha `locale.Message` com chave e dados; submensagens em `Parts` são formatadas no idioma corrente ao chamar `Render`. **Todas as chaves usadas em chamadas e mensagens precisam ser literais**, inclusive nos mapas de `Parts`: `go test ./internal/locale` rejeita chaves dinâmicas e confere automaticamente pares, parâmetros, variantes, duplicatas e usos sem tradução, sem lista manual de áreas.

## Como usar

O app abre numa tela inicial com 2 cartões: **Construir zonas**, o editor de zonas descrito abaixo, e **Popular zona**, o modo de áreas de spawn. O botão com a casa, no começo da barra de comandos, volta à tela inicial. Os tiles abertos, a câmera e o projeto são os mesmos nos 2 modos; atalhos, desfazer e refazer agem só no modo ativo.

1. Na tela inicial, clique em **Construir zonas**. Informe a pasta do cliente e o tile, e clique em **Abrir**.
2. Em **Nova zona**, digite o nome, escolha o tipo e clique em **Criar zona e desenhar**.
3. Clique sobre o mapa para marcar os vértices; Enter ou um clique no primeiro vértice fecha o polígono.
4. Ajuste altura, tipo e parâmetros no inspetor à direita e na janela **Altura da zona**.
5. Clique em **Compilar XML**, copie cada arquivo da janela e cole em `data/zone/` do servidor com o nome indicado.

O seletor **PT-BR / EN** fica sempre visível no alto da janela, abaixo do logotipo. Clique em **EN** para usar a interface em inglês ou em **PT-BR** para voltar ao português; a opção ativa fica destacada. A escolha é uma preferência pessoal salva na configuração do usuário (`%AppData%\ZoneBuilder\config.json`) e reaparece quando o app é aberto novamente. Sem preferência válida, o idioma inicial é pt-BR. A troca não modifica o projeto, os campos em edição nem o XML gerado.

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
| Linha na cor da aresta, em cada parede do prisma | o chão ao longo daquela aresta, do shape selecionado, só nas camadas que contam para a faixa (as outras camadas, como os andares de baixo de uma torre, ficam sem linha); segue a faixa ao arrastar a seta Z e some enquanto o shape é medido de novo |
| Aresta ou linha sólida | parte visível |
| Anéis horizontais nas paredes do prisma | o volume do shape: o prisma é oco, sem tampas nem preenchimento. Os anéis ficam a cada 64 de Z a partir do `zmin` e o passo dobra de longe, para nunca ficarem a menos de 6 px um do outro |
| Aresta ou linha tracejada, anéis fracos | parte do prisma enterrada ou atrás da cena, vista através do terreno |
| Pino com rótulo `topo +412` | chão mais alto do shape e a folga do topo nele |
| Pino com rótulo `piso −96` | chão mais baixo do shape e a folga do piso nele |
| Rótulo vermelho | folga negativa: o chão sai da faixa naquele ponto |

A pegada (tinta, hachuras e linhas de nível) só pinta chão: as faces voltadas para cima, vistas de cima. Paredes e tetos ficam com a cor original; só o contorno da zona continua desenhado sobre qualquer superfície que ele cruze, para mostrar onde a parede do prisma encontra o mapa. Com a câmera dentro do prisma, o mapa fica limpo: só os anéis nas paredes e a pegada no piso.

Clicar no rótulo de um pino leva a câmera ao ponto. O shader desenha a pegada de até 8 shapes e 128 pontos; acima disso, o inspetor diz "Pegada parcial: N shapes fora do desenho". Os números não têm esse limite.

### Números

O inspetor mostra, para o shape selecionado:

- **Chão mais baixo** e **Chão mais alto**, com `x y z`;
- **Folga do piso** (`chãoMin − zmin`) e **folga do topo** (`zmax − chãoMax`). Negativa quer dizer que fura e aparece como `−96 (fura)`;
- **Chão em N camadas**: o maior número de superfícies empilhadas numa coluna (terreno, piso de prédio, ponte);
- a área de chão, em unidades² e em % do chão sob o contorno: **Dentro da faixa**, **Acima do topo**, **Abaixo do piso**, **Excluída** (dentro de uma exclusão da zona e na faixa Z dela), **Em outras camadas** e **Sem chão** (quad invisível, tile não carregado, fora do mapa);
- **Outras camadas**, com o Z de cada uma: pisos de BSP ou mesh, ou o terreno, longe demais da faixa para contar (veja a regra das camadas abaixo).

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
- O seletor **Faixa nova**, no inspetor, logo abaixo de "Folga Z da faixa sugerida", escolhe a origem da faixa de um polígono, retângulo ou círculo novo: **Chão da área** (padrão, como acima) ou **Pontos clicados**, que dá exatamente o menor Z dos vértices menos a folga até o maior mais a folga, sem medir o chão. A prévia e a mensagem dizem "pelos pontos clicados". O tile inteiro sempre usa o chão da área. A escolha vale só na sessão.

**Regra das camadas** ([ADR 0006](docs/adr/0006-terreno-como-camada.md)). Um piso de BSP ou de static mesh só puxa a faixa quando cruza a faixa atual ou fica a até 1024 unidades dela; os outros aparecem como "outras camadas". O terreno segue a mesma regra, mas julgado em bloco: se alguma parte do terreno sob o contorno alcança a faixa, todo ele conta, e um morro no meio do retângulo continua puxando o topo. O terreno só vira outra camada quando nenhuma parte dele alcança a faixa e algum piso de BSP ou mesh alcança; se nenhum alcança, o terreno conta (fallback), e uma faixa longe de tudo ainda é ajustada por ele. Assim o telhado de uma torre ou uma caverna muito abaixo não esticam a faixa de uma zona de rua, e o terreno sob uma torre não puxa uma zona no topo dela. O chão excluído também não puxa. Com o botão **Static meshes** desligado, meshes não entram na medição.

No modo chão da área, os pisos internos de uma estrutura alta a até 1024 da faixa continuam puxando o piso, e depois da criação ainda podem gerar um aviso "chão abaixo do piso": a faixa é ajustada a partir dos pontos clicados, e os avisos são julgados de novo a partir da faixa ajustada, que alcança mais um andar (a consequência descrita abaixo). No topo da torre do 23_18, um polígono nasce com `z 8464..10340` e 1 aviso de folga do piso −990, de um piso interno a z 7474. Para uma zona só no andar onde ela foi desenhada, use **Faixa nova: Pontos clicados**.

A regra é aplicada uma vez, a partir da faixa de antes do clique. **Consequência aceita:** logo depois de um ajuste, uma camada que estava longe da faixa antiga mas fica perto da nova aparece como acima ou abaixo da faixa, com aviso; apertar o botão de novo a puxa para dentro. Repita só se essa camada deve mesmo fazer parte da zona.

## Zona de água

O servidor só conhece a água pela zona `water`, e o `zmax` dela é a superfície: decide onde o personagem nada, perde fôlego e onde a boia de pesca fica. O app gera essa zona a partir dos `WaterVolume` do mapa, sem desenhar à mão. Os termos (volume de água, topo, exata/aproximada) estão em [`GLOSSARY.md`](GLOSSARY.md).

1. Sem ferramenta armada, clique na água. Só o volume sob o clique fica selecionado, com o prisma destacado, e o status diz, por exemplo, `Água: 1 volume · topo -3780 (servidor -3810) · exata`. Clicar na superfície, no fundo visto através da água ou numa fonte de static mesh seleciona o volume; a margem seca não.
2. Ctrl+clique põe outro volume na seleção, ou tira um que já está nela; o status soma todos, por exemplo `Água: 8 volumes`. Esc ou clique fora da água limpa a seleção. Água sem `WaterVolume` dá `Esta água não tem WaterVolume no mapa` e não é selecionável. Um volume que o app não suporta (brush com cisalhamento, `SheerRate ≠ 0`) também não é selecionável: o clique, esquerdo ou direito, nele dá `Volume 22_22 WaterVolumeN não suportado: <motivo>`, e o log lista esses volumes ao abrir o tile.
3. Clique com o botão direito (sem arrastar) sobre a água: abre o menu com **Compilar zona de água**. Um volume fora da seleção passa a ser a seleção sozinho; um volume já selecionado mantém a seleção. Fora da água o botão direito não abre menu, e arrastar com ele continua girando a câmera. O menu fecha com Esc, com clique fora ou ao escolher o item. Com um polígono aberto, o item fica desabilitado: `Feche o polígono aberto antes de compilar a zona de água`.
4. O item cria as zonas no projeto (1 passo de desfazer: Ctrl+Z remove todas), compila só elas, sem mexer nas marcas de compilação da lista, e abre a janela de XML. As zonas ficam na lista, editáveis e salvas no `.zbproj`.

O que sai:

- **1 zona por topo**, com 1 polígono por volume: a envoltória convexa XY das faces, com o `zmin` e o `zmax` do próprio volume. O servidor usa o maior `zmax` dos shapes de uma zona, então topos diferentes nunca dividem a mesma zona.
- **Nome `[X_Y_<volume>]`**: o tile e o volume de menor nome do grupo, por exemplo `[22_22_WaterVolume0]`. Não colide com os nomes do datapack (`[22_22_water1]`).
- **Z = Z do cliente − 30** (`water.ServerZOffset`), como as zonas de água do datapack. **Pendente da medição em jogo** ([ADR 0005](docs/adr/0005-agua-30-abaixo-do-volume.md)). Por isso a zona compilada aparece no viewport com o topo 62 abaixo da água, e o status avisa: `No viewport o topo fica 62 abaixo da água (ADR 0005)`.
- Se o nome já existe no projeto, nada é criado e a versão do projeto é compilada: `Zona [22_22_WaterVolume0] já existe no projeto: compilada a versão do projeto`. Suas edições nunca são sobrescritas.

Avisos, no status e no log, sem bloquear a compilação:

| Aviso | Quando |
|---|---|
| `Água sobreposta: … cruza … (o servidor usa o maior topo)` | um volume selecionado cruza em XY e Z outro volume vivo, de topo diferente, que ficou fora da seleção |
| `Aproximada, parede ou topo inclinado: …` | o volume não é exato |
| `Passa do próprio tile: …` | o volume passa do tile dele; a zona entra no tile vizinho |

**Remova a zona de água antiga do datapack no mesmo lugar.** A zona nova tem outro nome e não a sobrescreve, e onde as duas se cruzam o servidor usa o maior `zmax` das duas. Enquanto a antiga estiver em `water.xml`, a nova não vale ali.

Fora do escopo: água sem `WaterVolume` (superfície decorativa, mar aberto feito com a ferramenta Tile inteiro) e outros volumes.

## Popular zona

Na tela inicial, clique em **Popular zona** (ou abra com `-mode populate`). A seção do mapa, o viewport, a câmera e o projeto são os mesmos do modo zonas. O inspetor mostra a lista **Áreas de spawn**, os problemas, o nome do **XML de spawn**, a **Área selecionada** e os **Pontos**. Os termos (área de spawn, ponto, raio, afastamento, célula livre, semente, pontos desatualizados, monstro de prévia, modo jogo) estão em [`GLOSSARY.md`](GLOSSARY.md), e a decisão de saída, no [ADR 0007](docs/adr/0007-ponto-fixo-em-vez-de-mesh.md).

1. **Área.** Escolha polígono, retângulo ou círculo na barra à esquerda e desenhe sobre o mapa, como nas zonas; o círculo vira polígono. A faixa Z sai do chão da área, e a janela de altura sobe, desce e ajusta o piso e o topo ao chão. A área nova se chama `area_<id>`, com raio 9 (o do monstro de prévia), afastamento 32 e respawn 60. Digite o **ID do NPC** e a **Quantidade**: enquanto forem 0, a área tem problema e não gera nem compila.
2. **Gerar.** Em **Pontos**, **Gerar** distribui a quantidade pedida nas células livres da área, em segundo plano. Os pontos ficam a 2 × raio um do outro, e a raio + afastamento das meshes e paredes; todas as meshes contam, mesmo com o botão **Static meshes** desligado. Se não couberem todos, o app gera os que cabem e avisa "cabem K de N", sem bloquear. **Regerar** troca a semente e descarta os ajustes manuais; Ctrl+Z volta os pontos anteriores.
3. **Ajustar pontos.** Cada ponto é um pino com o círculo do raio no chão. Arraste o pino para mover o ponto (ele cai na superfície sob o cursor), Delete apaga o ponto selecionado e **Adicionar pontos** põe um ponto a cada clique, até Esc. Mudar o contorno, a faixa, a quantidade, o raio ou o afastamento depois de gerar deixa os pontos **desatualizados**, o que bloqueia a compilação até gerar de novo; mover, apagar ou adicionar pontos não.
4. **Prévia.** O botão **Prévia**, no canto inferior direito do viewport, troca os pinos pelo monstro de prévia em cada ponto, em `Wait` e virado pelo heading. É sempre o mesmo modelo, seja qual for o id do NPC. Com a prévia ligada, os pontos continuam arrastáveis.
5. **Modo jogo.** **Jogar**, ao lado da prévia, põe um personagem no chão para andar entre os monstros. Veja [Modo jogo](#modo-jogo).
6. **Compilar.** **Compilar XML** abre a janela de XML com 1 arquivo para o projeto inteiro: o nome digitado em **XML de spawn** ou, em branco, o nome do projeto. Copie e grave em `data/spawn/` do servidor, e reinicie o servidor (`//reload_spawn` não relê o XML).

O XML tem 1 `<spawn name="[<área>_<i>]">` por ponto, com `<npc id count="1" respawn [respawn_rand] pos="x y z h" />`, em coordenadas do servidor. O monstro nasce e renasce sempre naquele ponto. `respawn_rand` só aparece quando é maior que 0. Problemas que bloqueiam: polígono com menos de 3 vértices ou que se cruza, faixa Z invertida, NPC ou quantidade menor que 1, respawn negativo, `respawn_rand` maior que o respawn, nome vazio ou repetido, área sem pontos, pontos desatualizados e ponto fora do mundo.

Atalhos, desfazer e refazer valem só para as áreas neste modo; o histórico das zonas não muda.

## Modo jogo

O botão **Jogar** fica no canto inferior direito do viewport, nos 2 modos. Ele põe o humano embutido no chão sob o centro da tela, com gravidade e colisão contra o terreno, o BSP sólido e as static meshes que bloqueiam no jogo (as flags de colisão de cada ator, com o padrão da classe). Meshes escondidas pelo botão **Static meshes** continuam bloqueando. Ao entrar, o app prepara as colisões dos tiles abertos por um instante ("Preparando as colisões do mapa…").

| Tecla ou gesto | Ação |
|---|---|
| W, A, S, D | andar |
| Shift | correr |
| Espaço | pular |
| F | voar ou parar de voar; no voo, E e Q sobem e descem, sem colisão |
| V | primeira ou terceira pessoa |
| arrastar | olhar em volta |
| Esc | sair e voltar à câmera de edição de antes |

No modo zonas, o contorno das zonas continua desenhado; no modo população, os monstros da prévia aparecem e não têm colisão. O modo ativo não recebe nenhuma entrada enquanto o jogo roda: a seleção e o histórico de desfazer ficam como estavam.

O personagem sobe rampas de terreno até uma inclinação de cerca de 49° (`n.z` 0,65) e degraus baixos; numa rampa mais íngreme, ele para ou escorrega. Isso difere do Play Map do UE2-Studio, que sobe rampas de 60°.

## Licenças de terceiros

- ANGLE (`third_party/angle`): BSD.
- Ícones Lucide (`internal/ui/icon/lucide`): ISC.
- Fonte Inter (`internal/ui/fonts`): SIL Open Font License.
