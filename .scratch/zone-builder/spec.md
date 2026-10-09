Status: resolved

# Zone Builder

## Problem Statement

Criar zonas para o servidor Lineage 2 é uma das tarefas mais difíceis do datapack. Hoje o desenvolvedor entra no jogo com um GM, anda até cada canto da área e grava a posição do personagem pelo painel `//zone_panel`. Esse fluxo tem limites que nenhum cuidado resolve:

- O personagem não alcança todos os cantos: telhados, paredes, penhascos, água funda, áreas bloqueadas pela geodata.
- O painel sempre gera `peace_zone`, sem parâmetros `set`, e cada ponto leva a própria faixa Z, embora o servidor use só a do último ponto.
- O XML sai no console do servidor, sem `<list>` nem `DOCTYPE`, e precisa ser copiado e ajustado à mão.
- Não há como ver as zonas que já existem sobre o mapa, nem saber se uma zona nova sobrepõe outra ou repete um nome.
- O servidor aceita dados ruins em silêncio: polígono que se cruza só gera log, nome duplicado sobrescreve outra zona, `type` com caixa errada derruba o resto do arquivo. O erro só aparece no jogo, depois de reiniciar o servidor.

## Solution

Um app desktop, o Zone Builder, que abre o mapa do cliente (`.unr`) texturizado num viewport 3D, como o UE2-Studio faz. Nele o desenvolvedor:

- Demarca a zona com o mouse: cada clique cai no ponto visível sob o cursor (terreno, BSP ou static mesh), em qualquer lugar do mapa.
- Edita nome, tipo e parâmetros num painel que só oferece valores que o servidor aceita.
- Vê os problemas de cada zona enquanto edita.
- Compila as zonas selecionadas para XML no formato exato que o servidor Java espera e que o `ZoneParser` dele carrega sem erro.

O app só lê arquivos `.unr` (e os pacotes de que eles dependem) e grava XML. Ele não lê o datapack nem altera o servidor Java; o formato e as regras do XML vêm do que o `ZoneParser` aceita.

As coordenadas do mapa são as mesmas do servidor, então o ponto clicado vai direto para o XML.

## User Stories

### Abrir o mapa

1. Como desenvolvedor do servidor, quero apontar o app para a pasta do cliente L2, para que ele encontre mapas, texturas e static meshes sozinho.
2. Como desenvolvedor do servidor, quero escolher um mapa pelo nome do tile (`X_Y`), para abrir a região onde a zona vai ficar.
3. Como desenvolvedor do servidor, quero abrir pacotes crus, `Lineage2Ver111` e `Lineage2Ver121`, para trabalhar com os mesmos clientes que o UE2-Studio abre.
4. Como desenvolvedor do servidor, quero abrir um `Lineage2Ver121` mesmo depois de renomeado, para não depender do nome original do arquivo.
5. Como desenvolvedor do servidor, quero que pacotes com pares ArVer/licensee diferentes no mesmo cliente abram sem erro, porque um cliente real mistura vários pares.
6. Como desenvolvedor do servidor, quero uma mensagem clara quando um pacote usa um container não suportado, para saber que o problema é a versão e não o arquivo.
7. Como desenvolvedor do servidor, quero ver o terreno, o BSP e os static meshes do mapa nos lugares certos, para reconhecer o local da zona.
8. Como desenvolvedor do servidor, quero que o mapa abra texturizado, com as camadas do terreno misturadas pelos alpha maps, para reconhecer caminhos, gramados e construções.
9. Como desenvolvedor do servidor, quero que texturas DXT1, DXT3, DXT5, RGBA8 e P8 apareçam, para que nenhum material fique sem textura por causa do formato.
10. Como desenvolvedor do servidor, quero que materiais Shader, FinalBlend e Combiner mostrem a textura de base, para que árvores e cercas apareçam como no jogo.
11. Como desenvolvedor do servidor, quero que folhagem e grades usem transparência por máscara, para enxergar através delas como no jogo.
12. Como desenvolvedor do servidor, quero abrir o tile junto com os vizinhos (até 3×3), para demarcar uma zona que cruza a borda entre tiles.
13. Como desenvolvedor do servidor, quero que tiles distantes sejam descarregados, para não esgotar a memória.
14. Como desenvolvedor do servidor, quero que o carregamento aconteça em segundo plano com progresso visível, para a janela continuar respondendo.
15. Como desenvolvedor do servidor, quero que o app lembre a pasta do cliente e os mapas recentes, para não configurar tudo a cada abertura.

### Navegar

16. Como desenvolvedor do servidor, quero uma câmera livre com WASD, Q/E, Shift para acelerar, arrastar para olhar e roda para aproximar, para usar os mesmos controles do UE2-Studio.
17. Como desenvolvedor do servidor, quero ver na barra de status a posição de mundo sob o cursor, para conferir coordenadas com o `//pos` do jogo.
18. Como desenvolvedor do servidor, quero digitar uma coordenada `x y z` e levar a câmera até ela, para ir direto a um ponto informado por um jogador ou por um log.
19. Como desenvolvedor do servidor, quero que selecionar uma zona leve a câmera até ela, para achar zonas pela lista.
20. Como desenvolvedor do servidor, quero que o viewport mantenha uma taxa de quadros fluida com vários tiles abertos, para demarcar sem travamentos.

### Demarcar

21. Como desenvolvedor do servidor, quero criar uma zona nova com nome e tipo, para começar a demarcação.
22. Como desenvolvedor do servidor, quero adicionar um vértice de polígono a cada clique no viewport, para traçar o contorno da zona.
23. Como desenvolvedor do servidor, quero que o vértice caia no ponto visível sob o cursor, seja terreno, telhado, interior BSP ou static mesh, para alcançar cantos onde o personagem não chega.
24. Como desenvolvedor do servidor, quero fechar o polígono com Enter ou clicando no primeiro vértice, para terminar o contorno.
25. Como desenvolvedor do servidor, quero criar um retângulo com 2 cliques em cantos opostos, para áreas retangulares.
26. Como desenvolvedor do servidor, quero desenhar um círculo por centro e raio e recebê-lo como polígono de N lados, porque o `circle` do servidor se comporta como quadrado.
27. Como desenvolvedor do servidor, quero criar exclusões (`banned_polygon`) dentro de uma zona com as mesmas ferramentas, para recortar áreas como lojas dentro de uma zona de não-comércio.
28. Como desenvolvedor do servidor, quero adicionar mais de um shape incluído na mesma zona, para cobrir áreas descontínuas com um único nome.
29. Como desenvolvedor do servidor, quero marcar `restart_point` e `PKrestart_point` com cliques, para definir onde jogadores voltam ao morrer.
30. Como desenvolvedor do servidor, quero arrastar um vértice e vê-lo voltar a encostar na superfície, para corrigir o contorno.
31. Como desenvolvedor do servidor, quero inserir um vértice no meio de uma aresta, para refinar o contorno sem redesenhar.
32. Como desenvolvedor do servidor, quero apagar um vértice, para simplificar o contorno.
33. Como desenvolvedor do servidor, quero mover um shape inteiro, para reposicionar uma zona sem redesenhá-la.
34. Como desenvolvedor do servidor, quero editar as coordenadas de um vértice pelo teclado, para ajustes exatos.
35. Como desenvolvedor do servidor, quero que a faixa Z seja sugerida pelo menor e maior Z dos vértices com uma folga configurável (padrão 256), para não calcular Z à mão.
36. Como desenvolvedor do servidor, quero editar `zmin` e `zmax` de cada shape, para zonas em andares ou cavernas.
37. Como desenvolvedor do servidor, quero recalcular a faixa Z a partir do chão sob os vértices, para reajustar depois de mover o contorno.
38. Como desenvolvedor do servidor, quero ver a zona como um prisma translúcido entre `zmin` e `zmax`, com arestas e vértices visíveis por cima da cena, para enxergar o volume real.
39. Como desenvolvedor do servidor, quero que cada tipo de zona tenha uma cor, para distinguir zonas sobrepostas.
40. Como desenvolvedor do servidor, quero desfazer e refazer qualquer edição, para experimentar sem medo.

### Ver e gerenciar

41. Como desenvolvedor do servidor, quero ver todas as zonas numa lista com nome, tipo e quantidade de problemas, para ter uma visão geral.
42. Como desenvolvedor do servidor, quero buscar zonas por nome, para achar uma entre centenas.
43. Como desenvolvedor do servidor, quero filtrar a lista por tipo, para focar num tipo de zona.
44. Como desenvolvedor do servidor, quero mostrar ou ocultar zonas, uma a uma e por tipo, para limpar o viewport.
45. Como desenvolvedor do servidor, quero escolher o tipo numa lista fechada com os 23 valores do enum do servidor, na caixa exata, para nunca gravar um tipo que derruba o arquivo.
46. ~~Como desenvolvedor do servidor, quero editar os parâmetros conhecidos do servidor (`enabled`, `default`, `target`, `affect_race`, `skill_name`, `damage_on_hp`, `blocked_actions` e os demais) com campos tipados e valores padrão visíveis, para não errar nome nem formato.~~ Removida em 2026-10-08: o foco do app são as coordenadas; parâmetros só entram como chave/valor (história 47).
47. Como desenvolvedor do servidor, quero adicionar parâmetros chave/valor, para os que o servidor exige (`residence`, `distribution_id`, `fishing_place_type`) e outros lidos por scripts.
48. Como desenvolvedor do servidor, quero renomear e apagar zonas, para manter o projeto organizado.
49. Como desenvolvedor do servidor, quero duplicar uma zona, para criar variações sem redesenhar.
50. Como desenvolvedor do servidor, quero salvar o trabalho num arquivo de projeto e reabri-lo depois, incluindo zonas incompletas, para trabalhar em várias sessões.

### Validar

51. Como desenvolvedor do servidor, quero um painel de problemas atualizado a cada edição, para corrigir erros antes de compilar.
52. Como desenvolvedor do servidor, quero que clicar num problema selecione a zona e leve a câmera até ela, para corrigir na hora.
53. Como desenvolvedor do servidor, quero ser avisado de polígonos com menos de 3 vértices, com arestas que se cruzam ou com vértices consecutivos repetidos, porque o servidor carrega esses polígonos e eles se comportam mal.
54. Como desenvolvedor do servidor, quero ser avisado quando `zmin` é maior que `zmax`, para não criar zonas que nunca contêm ninguém.
55. Como desenvolvedor do servidor, quero ser avisado de zonas sem shape incluído, porque o servidor gera NPE ao consultá-las.
56. Como desenvolvedor do servidor, quero ser avisado quando um nome se repete no projeto, porque o servidor sobrescreve sem avisar.
57. Como desenvolvedor do servidor, quero ser avisado quando faltam parâmetros obrigatórios de um tipo (`residence` em SIEGE e HEADQUARTER; `distribution_id` e `fishing_place_type` em FISHING; nome `residence_<id>` em RESIDENCE), para a zona funcionar no jogo.
58. Como desenvolvedor do servidor, quero ser avisado de coordenadas fora dos limites do mundo do servidor, para não criar zonas fora do mapa.
59. Como desenvolvedor do servidor, quero ver a zona com problema destacada no viewport, para achar o vértice errado.

### Compilar

60. Como desenvolvedor do servidor, quero selecionar quais zonas compilar, para exportar só o que está pronto.
61. Como desenvolvedor do servidor, quero que a compilação seja bloqueada enquanto houver problema numa zona selecionada, para nunca gravar XML que o servidor carrega errado.
62. Como desenvolvedor do servidor, quero que o XML saia com o cabeçalho, o `DOCTYPE` e o `<list>` que o servidor espera, para cair em `data/zone/` sem ajuste.
63. Como desenvolvedor do servidor, quero que cada polígono saia com o mesmo `zmin zmax` em todos os vértices, para o servidor aplicar a faixa que eu desenhei.
64. Como desenvolvedor do servidor, quero que os parâmetros saiam como `<set name="..." val="..." />`, porque o servidor ignora outras formas.
65. Como desenvolvedor do servidor, quero receber um arquivo por tipo de zona, com o nome sugerido com prefixo próprio do Zone Builder, para não sobrescrever arquivos de zona que já existem em `data/zone/`.
66. Como desenvolvedor do servidor, quero que a mesma entrada sempre gere o mesmo XML, byte a byte, para os diffs no git mostrarem só o que mudou.
67. Como desenvolvedor do servidor, quero ver o XML compilado numa janela, com um botão para copiar cada arquivo, para colá-lo no datapack sem o app gravar nada em disco. (Alterada em 2026-10-08: antes o app gravava os arquivos numa pasta de saída.)

## Implementation Decisions

### Linguagem e plataforma

- Go com Gio para a UI. Alvo: só Windows.
- Sem cgo. As chamadas EGL e GLES saem por carregamento dinâmico de DLL, como o próprio Gio faz no Windows.

### Leitura portada do UE2-Studio

- O Zone Builder porta para Go a lógica de leitura do UE2-Studio e não chama o código Rust. Os crates do UE2-Studio não expõem C ABI, e o caminho de leitura é puro (bytes → estruturas).
- Containers: cru, `Lineage2Ver111` (XOR 0xAC) e `Lineage2Ver121` (XOR com o byte baixo da soma do nome do arquivo em minúsculas, com recuperação da chave pelo magic). Os demais containers geram o erro "versão de container não suportada" com o número da versão.
- Pares ArVer/licensee: os medidos pelo UE2-Studio, 117/0 a 133/40, com os mesmos gates de licensee:
  - o bloco nativo de material antes das mips;
  - o segundo array de atores no Level;
  - o campo extra nas superfícies BSP.
- Texturas: DXT1, DXT3, DXT5, RGBA8 (gravado como BGRA) e G16 como no UE2-Studio, mais P8 com Palette, que o UE2-Studio não decodifica.
- Resolução de dependências: por nome de pacote nas pastas de assets do cliente (Maps, StaticMeshes, Textures, SysTextures, System), com cache por pacote. A raiz do cliente é a pasta acima de `Maps`.

### Módulos

- **Pacote:** container, reader, compact index, header, tabelas de nomes/imports/exports, propriedades com tag e resolução de imports entre pacotes.
- **Textura:** mips e decodificação.
- **Objetos Unreal:**
  - Level, Model (BSP), StaticMesh, TerrainInfo e atores com transform;
  - grafo de material: Shader, FinalBlend e Combiner seguidos até 8 níveis até a primeira Texture, com `Skins[i]` do ator valendo mais que `Materials[i]` da mesh.
- **Cena:** o primeiro seam. Ver a seção própria abaixo.
- **Renderizador:** consome a cena; não tem seam de teste. Ver a seção própria abaixo.
- **Câmera:** câmera livre e ray a partir do cursor.
- **Documento de zonas:** o segundo seam. Ver a seção própria abaixo.
- **XML de zonas:** compilação para o formato que o `ZoneParser` aceita. Usado por dentro do Documento.
- **Projeto:** leitura e gravação do arquivo de projeto.
- **UI:**
  - painéis Gio de lista de zonas, propriedades, ferramentas, problemas e status;
  - ferramentas do viewport que traduzem cliques em comandos do Documento.
- **CLI de diagnóstico:** abre pacotes e lista exports; varre um cliente inteiro.

### Seam 1: Cena

**Interface:** `Load(raizDoCliente, tiles) → Scene` e `Scene.Pick(ray) → Hit`, com `Hit` contendo `x y z` em coordenadas de mundo e a superfície atingida.

`Load` monta a cena em memória de CPU, sem GPU:
- **Batches:** um por (textura, máscara, modo de render), com índices uint32.
- **Terreno:**
  - heightmap G16 com `TerrainScale`, `QuadVisibilityBitmap` e `EdgeTurnBitmap`;
  - fallback por `MapX/MapY` quando a escala vem quebrada;
  - camadas com UV por camada (rotação, pan, escala) e máscara do canal R do alpha map.
- **BSP:** fan por nó, com as superfícies invisíveis, de portal e de backdrop puladas.
- **Static meshes:** atores `StaticMeshActor`, `MovableStaticMeshActor`, `L2MovableStaticMeshActor`, `Mover` e `L2NMover`, com transform `Location - PrePivot`, rotação GLM-euler e escala `DrawScale3D·DrawScale`.
- **Filtros:** região maior que 2 tiles e fora do mapa, como no UE2-Studio.

`Pick`:
- DDA no grid do terreno e Möller–Trumbore com early-out por AABB nas superfícies BSP e nas meshes.
- Devolve o acerto mais próximo.

**Coordenadas:**
- O espaço Unreal é o espaço de mundo do servidor: o tile `X_Y` começa em `((X-20)·32768, (Y-18)·32768)`.
- A cena guarda coordenadas absolutas e expõe uma origem de rebase para o renderizador manter precisão de float.

### Renderizador (sem seam de teste)

- **Janela e contexto:**
  - janela Gio com renderizador customizado;
  - contexto EGL via ANGLE (`libEGL.dll` e `libGLESv2.dll` distribuídas junto do executável);
  - a cena 3D é desenhada só na área do widget de viewport e os painéis do Gio por cima, no mesmo frame.
- **Passes, na ordem do UE2-Studio:** Opaque, Masked (corte em alfa 0,5), TerrainLayer (blend alfa, profundidade `>=` sem escrita), Translucent, Brighten, Modulated, Additive, Water, Overlay.
- **Overlay de zonas:** prisma translúcido entre `zmin` e `zmax`, arestas e alças de vértice desenhadas por cima da cena.
- **Desempenho:** frustum culling por batch, com batches divididos por setor do tile.
- **Decisões que o primeiro marco grava em ADR:**
  - profundidade: Z reverso se o ANGLE expuser `GL_EXT_clip_control`, senão a alternativa escolhida;
  - texturas: DXT enviado nativo ou decodificado na CPU.

### Seam 2: Documento de zonas

**Interface:**
- `Apply(comando)`, `Undo()`, `Redo()`
- `Problems() → []Problem`
- `Compile(seleção) → arquivos XML`
- `Save` / `Load` do projeto

Comandos: criar zona, renomear, definir tipo, definir e remover parâmetro, adicionar shape (incluído ou banido), adicionar, mover, inserir e remover vértice, mover shape, definir faixa Z, adicionar e remover restart point, apagar zona, duplicar zona.

A UI só produz comandos; ela nunca altera zonas direto. Por isso o desfazer cobre toda edição.

**Modelo de zona:**
- Zona: nome, tipo, parâmetros `set` em ordem, shapes incluídos, shapes banidos, `restart_point`, `PKrestart_point`.
- Shape: polígono ou retângulo, com uma única faixa Z (`zmin`, `zmax`).
- O círculo da UI vira polígono antes de chegar ao Documento.

**Tipos:** lista fechada com os 23 valores do enum `ZoneType` do servidor, na caixa exata: SIEGE, RESIDENCE, HEADQUARTER, FISHING, water, battle_zone, damage, instant_skill, mother_tree, peace_zone, poison, ssq_zone, swamp, no_escape, no_landing, no_restart, no_summon, dummy, offshore, epic, fun, buff_store, JUMPING.

**Parâmetros:**
- Chave/valor livres, sem catálogo de tipos: qualquer nome é aceito, exceto os que o `ZoneParser` usa para os próprios dados (`name`, `type`, `territory`, `restart_points`, `PKrestart_points`).

**Validação.** Cada regra produz um `Problem` com zona, shape ou vértice e mensagem:
- Polígono com 3 ou mais vértices.
- Polígono simples, com todos os pares de arestas não adjacentes testados. Isso é mais rígido que o servidor, que pula a aresta 0→1.
- Sem vértices consecutivos iguais.
- Retângulo com exatamente 2 cantos.
- `zmin <= zmax`.
- Pelo menos 1 shape incluído por zona.
- Tipo dentro do enum.
- Nome único no projeto.
- Parâmetros obrigatórios por tipo.
- Coordenadas dentro de X ∈ [−163840, 229375] e Y ∈ [−262144, 294911].

O app não lê XML de zona: a única entrada de zonas é o arquivo de projeto. As regras de leitura do `ZoneParser` (Z padrão −32768..32767 quando a coords tem menos de 4 números, faixa Z do polígono vinda da última coords, `circle` tratado como quadrado `c ± r`) só servem para decidir o que o compilador emite.

**Compilação:**
- Cabeçalho `<?xml version='1.0' encoding='utf-8'?>`, `<!DOCTYPE list SYSTEM "zone.dtd">` e `<list>`.
- Indentação com tab.
- Um arquivo por tipo, com nome sugerido de prefixo próprio do Zone Builder, mostrado numa janela com botão de copiar; o app não grava XML em disco.
- Zonas em ordem de nome e parâmetros na ordem do documento.
- Polígono com coords de 4 números e a mesma faixa Z em todos os vértices.
- Exclusões como `banned_polygon`.
- `circle` nunca é emitido.
- Saída determinística.
- A compilação falha, sem mostrar nada, se alguma zona selecionada tiver problema.

### Projeto

O arquivo de projeto (JSON) guarda:
- caminho do cliente;
- tiles abertos;
- zonas, inclusive incompletas;
- cor e visibilidade por zona.

As preferências do usuário (caminhos recentes) ficam fora do projeto, na configuração do usuário.

### CLI de diagnóstico

Abre um pacote e lista seus exports; com uma pasta de cliente, varre todos os `.unr`, `.utx` e `.usx` e relata cada falha com o nome do pacote e o motivo.

## Testing Decisions

Um bom teste exercita comportamento externo por um dos 2 seams e nomeia o bug concreto que pegaria. Não se testam detalhes internos (ordem de campos privados, chamadas intermediárias), constantes contra si mesmas nem textos de widgets. Uma regra, um teste.

### Seam Cena

Os testes rodam contra o cliente real, apontado por variável de ambiente, e são pulados quando ela não existe.

- **Varredura:** todo `.unr`, `.utx` e `.usx` do cliente Majestic World abre sem erro de leitura. Pega quebra de gate de versão/licensee.
- **Ver121 renomeado:** abre pela recuperação da chave. Pega regressão na derivação da chave.
- **Contagem de exports:** em 1 mapa, 1 `.utx` e 1 `.usx` de amostra, bate com o que o UE2-Studio relata. Pega deslocamento nas tabelas.
- **Picking:** em pontos de referência conhecidos de Giran, com coordenadas anotadas via `//pos` no jogo, `Pick` de um ray vertical devolve `x y z` a menos de 16 unidades. Pega erro de transform, de escala do terreno ou de origem do tile, e confirma o Z 1:1.
- **Decodificação de textura:** blocos DXT1/3/5 e P8 reais do cliente decodificam para os pixels esperados. Pega troca de canal BGRA e erro de alfa 1-bit do DXT1.

### Seam Documento

Testes sem GPU nem UI.

- **Polígonos que o servidor deixa passar:** auto-interseção na aresta 0→1 e vértices consecutivos iguais geram `Problem`.
- **Zona sem shape incluído:** gera `Problem`.
- **Nome duplicado no projeto:** gera `Problem`.
- **Tipo com caixa errada:** `Siege` em vez de `SIEGE` não pode ser definido.
- **Faixa Z na saída:** num polígono compilado, todas as coords trazem o mesmo `zmin zmax` de 4 números. Pega a regressão que faria o servidor aplicar a faixa errada, já que ele usa a da última coords.
- **Saída determinística:** compilar 2 vezes o mesmo documento gera bytes idênticos.
- **Bloqueio da compilação:** com uma zona selecionada inválida, a compilação falha sem gerar arquivo.
- **Desfazer/refazer:** uma sequência de comandos seguida do mesmo número de `Undo` devolve o documento inicial; `Redo` reaplica.
- **Projeto:** salvar e carregar devolve o documento idêntico, inclusive com zona incompleta.

### Sem teste automatizado

- Renderizador e UI. São conferidos rodando o app: comparação lado a lado com o modo Textured do UE2-Studio em 3 vistas e demarcação de uma zona completa só com o mouse.
- Aceitação final: o XML compilado de um projeto com 1 zona de cada um dos 23 tipos carrega no servidor Java, sem nenhuma alteração nele, sem `invalid territory data`, `Empty territory` ou exceção; `//zone_check` responde conforme o desenho dentro da zona, fora dela e dentro da exclusão.

### Prior art

O repositório ainda não tem testes. A referência de estilo é a regra do UE2-Studio: um teste só existe quando nomeia o bug que pega, com casos tirados de dados reais do cliente.

## Out of Scope

- Containers `Lineage2Ver120`, 211/212 (Blowfish) e 411-414 (RSA + zlib). Ficam para quando um cliente alvo exigir.
- Linux e macOS.
- Escrita em qualquer pacote do cliente (`.unr`, `.utx`, `.usx`).
- Geração ou leitura de geodata.
- Iluminação (lightmaps, vertex colors), skeletal meshes, emitters e sons.
- Ler, mostrar, importar ou editar zonas que já existem no datapack. O datapack só serviu de referência para entender o formato que o Java espera.
- `domains.xml`, `restart_points.xml` e territórios de spawn, que têm formatos de saída próprios.
- Qualquer mudança no servidor Java. O app só abre `.unr` e gera XML válido para o servidor como ele é hoje.

## Further Notes

- **Ordem de entrega:**
  1. Spike de renderização (Gio + ANGLE, decisões de profundidade e DXT em ADR).
  2. Leitura de pacotes com a CLI de diagnóstico.
  3. Geometria e picking.
  4. Texturização.
  5. Vários tiles e desempenho.
  6. Ferramentas de demarcação.
  7. Gerenciamento de zonas.
  8. Validação e compilação.

  O plano completo, com critérios de pronto por marco, está em `docs/plan.md`.
- **Decisões tomadas por padrão nesta spec,** entre as que o plano deixou em aberto:
  - só Windows;
  - containers só os que o UE2-Studio abre;
  - saída em um arquivo por tipo com prefixo próprio.
- **Nomes que colidem com o datapack:** o app não conhece as zonas existentes, então não detecta um nome que já existe no datapack. O servidor sobrescreve sem avisar; escolher nomes únicos (por exemplo com um prefixo próprio) fica com o usuário.
- **Bug do servidor:** `Circle.isInside` é sempre verdadeiro, por isso o app nunca emite `circle`.
- **Sem reload no servidor:** o servidor não recarrega zonas em runtime, então cada teste no jogo exige reiniciar o servidor.
