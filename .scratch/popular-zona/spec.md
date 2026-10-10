Status: ready-for-agent

# Popular zona: assistente de spawn de monstros para zonas de farm

Plano de origem: `docs/plan-populacao.md` (marcos P0-P5). Esta spec segue as decisões aprovadas pelo usuário em 2026-10-09: só pontos fixos, o monstro de prévia é o `death_knight_wizard_m00`, o humano do UE2-Studio está na escala certa, o UE2-Studio é só leitura, e o modo jogo existe nos dois modos.

## Problem Statement

Hoje, para povoar uma zona de farm, o usuário escreve o XML de spawn à mão ou anda com o personagem no jogo anotando `//pos`. Escolher onde cada monstro fica é tentativa e erro. Um monstro dentro de uma cerca, de um tronco ou de uma pedra só aparece depois de reiniciar o servidor, e o servidor não relê spawns em runtime.

O servidor também não resolve isso sozinho. Uma `<mesh>` com `count` faz o servidor sortear um ponto novo a cada respawn, olhando só a geodata: não há espaçamento entre monstros, o raio de colisão não conta e a static mesh que a geodata não registra fica de fora. O usuário não vê antes como a zona vai ficar e não tem como aprovar uma distribuição.

O Zone Builder já abre o mapa do cliente em 3D, sabe onde está o chão e onde estão as static meshes, e já compila XML para o servidor. Falta um modo de uso para isso: demarcar a área, pedir 50 monstros, ver os monstros no lugar, andar entre eles como no jogo e só então gerar o XML.

## Solution

1. O app abre numa **tela inicial** com 2 opções: "Construir zonas", o editor de hoje, e "Popular zona", o novo modo. Um botão "Início" volta para essa tela. Os tiles abertos, a câmera e o projeto são os mesmos nos 2 modos.
2. No modo população, o usuário desenha uma **área de spawn** sobre o mapa com as ferramentas de polígono, retângulo e círculo que já conhece. Depois informa o id do NPC, a quantidade, o respawn, o `respawn_rand`, o raio de colisão e o afastamento das meshes.
3. **Gerar** distribui os monstros pelo chão livre da área. O chão livre exclui:
   - chão de static mesh;
   - chão inclinado demais;
   - chão debaixo d'água;
   - pontos perto de tronco, cerca, pedra ou parede, conforme o raio e o afastamento.

   Copa de árvore acima da cabeça do monstro não atrapalha. Os pontos saem espalhados, sem cara de grade, e nunca se sobrepõem. Se não couberem todos, o app diz quantos couberam. A mesma semente gera sempre os mesmos pontos, e **Regerar** tenta outra.
4. O usuário pode arrastar, apagar e adicionar pontos à mão, com desfazer.
5. **Prévia** mostra o death knight wizard embutido em cada ponto, em idle, com os pés no chão e virado para o heading que vai para o servidor.
6. **Jogar** coloca o humano embutido no chão e deixa o usuário andar, correr, pular e voar entre os monstros, com a colisão do mapa, como no Play Map do UE2-Studio. O botão existe também no modo de zonas.
7. **Compilar XML** gera um `<spawn>` por ponto, com `pos` fixo, num arquivo próprio de nome livre. O arquivo aparece na janela de XML com o botão de copiar. Vai para `data/spawn/` do servidor, e o monstro nasce e renasce exatamente onde a prévia mostrou.

## Termos

Entram no `GLOSSARY.md`, numa seção "População", no ticket de docs.

| Termo | Definição |
| --- | --- |
| Área de spawn | Polígono do modo população, com faixa Z, id do NPC, quantidade, respawn, raio, afastamento, semente e pontos. Não é uma zona: não tem tipo, não compila para o XML de zona e não tem exclusões, porque o servidor não suporta exclusão em spawn. |
| Ponto de spawn | Um monstro da área: `x y z heading` em coordenadas do servidor. Vira 1 `<spawn>` com `pos`. |
| Raio | Raio de colisão do monstro. Afasta os pontos da borda da área e um ponto do outro (no mínimo 2 × raio). O padrão é o medido na mesh do monstro de prévia. |
| Afastamento | Distância a mais, além do raio, que o ponto guarda das static meshes e das paredes. O padrão é 32. _Evite_: folga (é a do piso e do topo), margem (é a dos ajustes de faixa). |
| Célula livre | Célula de 16 unidades dentro da área, com chão de terreno ou BSP pouco inclinado, fora d'água e sem obstáculo a menos de raio + afastamento. |
| Semente | Número que determina o sorteio. A mesma semente, com as mesmas entradas, dá os mesmos pontos. |
| Pontos desatualizados | A área mudou (contorno, faixa, quantidade, raio ou afastamento) depois da última geração. Bloqueia a compilação. |
| Monstro de prévia | O modelo embutido que a prévia desenha em todo ponto, seja qual for o id: o `death_knight_wizard_m00`. |
| Modo jogo | O port do Play Map: cápsula com gravidade e colisão, controlando o humano embutido. |

## User Stories

### Tela inicial e modos

1. Como usuário, quero escolher na abertura do app entre construir zonas e popular zona, para ir direto à tarefa que vim fazer.
2. Como usuário, quero ler uma frase curta em cada opção da tela inicial, para entender a diferença entre os 2 modos sem abrir a documentação.
3. Como usuário, quero um botão "Início" para voltar à tela inicial, para trocar de modo sem fechar o app.
4. Como usuário, quero que os tiles abertos continuem abertos ao trocar de modo, para não esperar o mapa carregar de novo.
5. Como usuário, quero que a câmera fique onde estava ao trocar de modo, para criar zonas e spawns no mesmo lugar.
6. Como usuário, quero que o modo de zonas funcione exatamente como hoje, para não reaprender nada do que já uso.
7. Como usuário, quero um único projeto com as zonas e as áreas de spawn, para salvar e abrir o trabalho de um mapa num arquivo só.
8. Como usuário, quero abrir os projetos que já salvei, sem perder nenhuma zona, para que a atualização não quebre meu trabalho.
9. Como usuário de um binário antigo, quero que um projeto novo seja recusado em vez de aberto pela metade, para que salvar por cima não apague minhas áreas de spawn.
10. Como desenvolvedor, quero uma flag de linha de comando que abra direto num dos modos, para os smoke tests não passarem pela tela inicial.

### Área de spawn

11. Como usuário, quero desenhar a área de spawn clicando no chão, como faço com as zonas, para demarcar o lugar exato da farm.
12. Como usuário, quero desenhar a área com retângulo e círculo também, para demarcar campos regulares com 2 cliques.
13. Como usuário, quero editar os vértices da área (arrastar, inserir no meio de uma aresta, apagar e mover a área inteira), para corrigir o contorno sem redesenhar.
14. Como usuário, quero desfazer e refazer toda edição de área e de pontos, para experimentar sem medo.
15. Como usuário, quero que a faixa Z da área venha do chão sob o contorno, como nas zonas, para que a área fique na camada certa (debaixo ou em cima de uma ponte) sem eu calcular.
16. Como usuário, quero ajustar a faixa Z da área na janela de altura que já conheço, para escolher a camada quando a sugestão errar.
17. Como usuário, quero dar um nome à área, para reconhecê-la na lista e no XML (`[<nome>_<i>]`).
18. Como usuário, quero informar o id do NPC, para o servidor nascer o monstro certo.
19. Como usuário, quero informar quantos monstros quero na área, para controlar a densidade da farm.
20. Como usuário, quero informar o respawn e o `respawn_rand` em segundos, com 60 e 0 como padrão, para controlar o ritmo da farm.
21. Como usuário, quero informar o raio do monstro, com o valor medido do monstro de prévia como padrão, para que monstros maiores fiquem mais longe uns dos outros e das meshes.
22. Como usuário, quero informar o afastamento das meshes, com 32 como padrão, para deixar mais ou menos espaço livre entre os monstros e o cenário.
23. Como usuário, quero uma lista com as áreas do projeto, para navegar entre várias farms do mesmo mapa.
24. Como usuário, quero que selecionar uma área na lista leve a câmera até ela, para achar a área no mapa.
25. Como usuário, quero mostrar e ocultar cada área, para limpar o viewport enquanto trabalho em outra.
26. Como usuário, quero duplicar uma área, para criar uma variação (outro id, outra quantidade) sem redesenhar.
27. Como usuário, quero apagar uma área, com desfazer, para descartar uma tentativa.

### Distribuição

28. Como usuário, quero gerar os pontos com um botão, para ver na hora como a área fica povoada.
29. Como usuário, quero que nenhum monstro nasça em cima de static mesh, para não ter monstro em pedra, telhado ou mesa.
30. Como usuário, quero que nenhum monstro nasça a menos de raio + afastamento de tronco, cerca, pedra ou parede, para que ninguém nasça preso no cenário.
31. Como usuário, quero que a copa de uma árvore acima da cabeça do monstro não bloqueie o chão embaixo dela, para aproveitar a sombra da floresta.
32. Como usuário, quero que nenhum monstro nasça debaixo d'água, para não povoar lagos por engano.
33. Como usuário, quero que nenhum monstro nasça em chão inclinado demais para andar, para não ter monstro pendurado em barranco.
34. Como usuário, quero que os monstros fiquem dentro da área com o corpo inteiro (a pelo menos 1 raio da borda), para que a área desenhada seja respeitada.
35. Como usuário, quero que 2 monstros nunca fiquem a menos de 2 raios um do outro, para não ter monstros encavalados.
36. Como usuário, quero pontos espalhados de forma natural, sem grade, para a farm parecer feita à mão.
37. Como usuário, quero que gerar de novo com a mesma semente dê os mesmos pontos, para que reabrir o projeto não mude a farm.
38. Como usuário, quero regerar com outra semente, para tentar outra distribuição quando não gosto da atual.
39. Como usuário, quero um aviso "cabem K de N" quando a área não comporta a quantidade, para saber que preciso aumentar a área ou reduzir o raio.
40. Como usuário, quero um aviso quando a área não tem nenhuma célula livre, para entender por que nada foi gerado.
41. Como usuário, quero ver no inspetor a área de chão livre, o espaçamento médio e o menor espaçamento, para julgar a densidade com números.
42. Como usuário, quero ver cada ponto como um pino com o círculo do raio no chão, para conferir os afastamentos antes da prévia.
43. Como usuário, quero arrastar um ponto e que ele caia no chão sob o cursor, para ajustar um monstro sem regerar tudo.
44. Como usuário, quero apagar um ponto, para tirar um monstro de um lugar ruim.
45. Como usuário, quero adicionar um ponto com um clique, para pôr um monstro onde o sorteio não pôs.
46. Como usuário, quero ser avisado de que regerar descarta meus ajustes manuais, e poder desfazer, para não perder trabalho sem querer.
47. Como usuário, quero que mudar o contorno, a faixa, a quantidade, o raio ou o afastamento marque os pontos como desatualizados, para nunca compilar pontos que não correspondem à área.
48. Como usuário, quero que um ponto desatualizado apareça diferente no viewport, para ver que preciso regerar.

### Prévia

49. Como usuário, quero ligar a prévia com um botão flutuante no viewport, para trocar os pinos pelos monstros.
50. Como usuário, quero ver um monstro de verdade em cada ponto, texturizado e animado em idle, para julgar a farm como ela vai ficar no jogo.
51. Como usuário, quero que o monstro fique com os pés no chão, para que a prévia não engane sobre a altura.
52. Como usuário, quero que o monstro fique virado para o heading que vai no XML, para ver a direção real em que ele vai nascer.
53. Como usuário, quero que a prévia continue fluida com centenas de monstros, para povoar áreas grandes.
54. Como usuário, quero que o monstro de prévia seja o mesmo para qualquer id, sabendo que é só referência de tamanho e posição.

### Modo jogo

55. Como usuário, quero entrar no modo jogo com um botão flutuante no viewport, para ver a farm da altura do personagem.
56. Como usuário, quero começar no chão sob o centro da tela, para entrar no jogo onde estou olhando.
57. Como usuário, quero andar com WASD, correr com Shift e pular com Espaço, como no Play Map do UE2-Studio, para usar os mesmos controles.
58. Como usuário, quero ser barrado por cercas, pedras, troncos e paredes que bloqueiam no jogo, para sentir os caminhos reais da farm.
59. Como usuário, quero subir rampas até a inclinação que o jogo permite e escorregar nas mais íngremes, para ver por onde o personagem passa.
60. Como usuário, quero voar com F, sem colisão, para sair de um buraco ou olhar a farm de cima.
61. Como usuário, quero alternar entre primeira e terceira pessoa com V, para ver o personagem ou o que ele vê.
62. Como usuário, quero que a câmera em terceira pessoa encurte perto das paredes, para não ver o mapa por dentro.
63. Como usuário, quero olhar arrastando o mouse, como no voo de hoje, para não perder o cursor.
64. Como usuário, quero ver o humano animado (parado, correndo, caindo e pulando), para julgar a escala dos monstros ao lado dele.
65. Como usuário, quero ver os monstros da prévia enquanto ando e atravessar os monstros, para julgar a densidade sem gameplay.
66. Como usuário, quero sair do modo jogo com Esc e voltar à câmera de edição onde estava, para continuar editando.
67. Como usuário do modo de zonas, quero o mesmo modo jogo, com o contorno das zonas desenhado, para conferir uma zona andando.
68. Como usuário do modo de zonas, quero que entrar e sair do modo jogo mantenha a seleção e o histórico de desfazer, para não perder o contexto da edição.

### Validação e compilação

69. Como usuário, quero que os problemas da área apareçam no painel de problemas, com um clique levando até ela, para corrigir antes de compilar.
70. Como usuário, quero que uma área com contorno inválido (menos de 3 vértices ou auto-interseção) bloqueie a compilação, para não gerar XML de uma área que eu não queria.
71. Como usuário, quero que id ≤ 0, quantidade < 1, nome vazio ou repetido e `respawn_rand > respawn` bloqueiem a compilação, porque o servidor perde o arquivo inteiro nesses casos.
72. Como usuário, quero que pontos desatualizados ou fora dos limites do mundo bloqueiem a compilação, para que o XML seja sempre a distribuição que eu vi.
73. Como usuário, quero que os avisos de distribuição (cabem K de N, sem célula livre) não bloqueiem a compilação, para compilar os pontos que couberam.
74. Como usuário, quero compilar todas as áreas num arquivo só, com nome livre (padrão: o nome do projeto), para soltar um arquivo próprio em `data/spawn/` sem mexer nos arquivos de tile do datapack.
75. Como usuário, quero um `<spawn>` por ponto com `pos` e `count="1"`, no estilo `[Den_of_Evil_N]` do datapack, para o monstro renascer sempre no mesmo ponto aprovado.
76. Como usuário, quero que o Z do XML esteja em coordenadas do servidor, para o servidor escolher a camada certa ao ajustar pela geodata.
77. Como usuário, quero que o heading nunca seja 0, porque o servidor ignora o 0 e o monstro sairia virado para o lado errado.
78. Como usuário, quero que o XML apareça na janela de XML com o botão de copiar, como as zonas, para colar no servidor.
79. Como usuário, quero que compilar 2 vezes o mesmo projeto dê exatamente o mesmo XML, para o diff no repositório do servidor só mostrar o que mudou.

### Monstro embutido (desenvolvedor)

80. Como desenvolvedor, quero uma CLI offline que extrai o monstro de prévia direto do cliente, para regenerar o asset sem depender do UE2-Studio.
81. Como desenvolvedor, quero que a CLI leia a mesh, a animação e as skins pelos nomes de pacote, para trocar de monstro só mudando os argumentos.
82. Como desenvolvedor, quero que a CLI imprima o raio e a altura medidos do monstro, para fixar os padrões do app a partir da mesh real.
83. Como desenvolvedor, quero que o app leia o `human.bin` do UE2-Studio sem conversão, para reaproveitar o personagem aprovado sem tocar no app Rust.
84. Como desenvolvedor, quero que decodificar e recodificar o `human.bin` dê os mesmos bytes, para provar que o formato foi portado sem perda.
85. Como desenvolvedor, quero que o app em runtime não leia nada do cliente para o monstro e o humano, para a prévia funcionar com qualquer cliente aberto.
86. Como desenvolvedor, quero documentada a procedência dos 2 modelos e o comando de regeneração, para quem vier depois.

### Idiomas

87. Como usuário em inglês, quero toda a interface nova traduzida, para usar o modo população no meu idioma.
88. Como usuário, quero as contagens flexionadas ("1 monstro", "2 monstros", "cabem 1 de 50"), como no resto do app.

## Implementation Decisions

### D1. Modos e tela inicial

- O estado do editor sai das variáveis locais do laço de eventos para um tipo de modo com 3 valores: início, zonas e população. Cada modo é dono do seu editor e do seu painel no inspetor. A seção do mapa, o viewport, a câmera, a barra de comandos e o projeto ficam compartilhados.
- A tela inicial cobre o viewport com 2 cartões no estilo dos cartões flutuantes de hoje, com ícone Lucide, título e uma frase. O botão "Início" fica na barra de comandos.
- Os atalhos de teclado e os eventos do viewport vão só para o modo ativo. Desfazer e refazer usam o histórico do modo ativo.
- A flag `-mode zones|populate` pula a tela inicial. Sem a flag, o app sempre abre na tela inicial, mesmo com `-project`.

### D2. Documento de spawn

- Módulo novo `spawn`, no molde de `zone.Document`: estado privado, mutação só por `Apply(Command)`, histórico por snapshot, `Problems()` em cache e `MarshalJSON`/`UnmarshalJSON` com validação.
- `Area`: ID, nome, contorno (lista de pontos XY em coordenadas do servidor, sempre polígono), `ZMin`, `ZMax`, id do NPC, quantidade, respawn, `respawn_rand`, raio, afastamento, semente, pontos, impressão das entradas da última geração e oculta.
- O retângulo e o círculo viram polígono no documento: o spawn só precisa de contenção em XY. Os comandos de vértice são os de `zone`, com outro alvo.
- `Point`: `X, Y, Z, Heading` inteiros, em coordenadas do servidor.
- Comandos: criar, apagar e duplicar área; editar vértices e mover; mudar a faixa e os parâmetros; `SetPoints` (resultado da geração, junto com a semente e a impressão das entradas); mover, apagar e adicionar ponto.
- **Pontos desatualizados:** a impressão das entradas (contorno, faixa, quantidade, raio, afastamento, semente) é gravada em `SetPoints`. A área fica desatualizada quando a impressão atual difere da gravada. Mover, apagar ou adicionar ponto não mexe na impressão.
- **Problemas que bloqueiam:** contorno com menos de 3 vértices ou com auto-interseção (a mesma regra mais rígida de `zone`), vértices consecutivos repetidos, `zmin > zmax`, id ≤ 0, quantidade < 1, `respawn < 0`, `respawn_rand > respawn`, nome vazio ou repetido no documento, área sem pontos, pontos desatualizados e ponto fora de X ∈ [−163840, 229375] e Y ∈ [−262144, 294911].
- **Avisos** (gravados na geração, não bloqueiam): cabem K de N, e nenhuma célula livre.
- `Compile(fileName) (File, error)`, com `*BlockedError` como em `zone`.

### D3. Distribuição

- Módulo novo `placement`, puro (sem GL e sem Gio): `Distribute(g Geometry, r Request) Result`.
- `Geometry` é a interface de leitura da cena, implementada por `scene.World`. Ela entrega os triângulos de uma caixa em coordenadas do servidor, com a superfície de cada um (terreno, BSP ou mesh), e os volumes de água da caixa. É um método novo do `World`, irmão de `Floor`, que entrega **todos** os triângulos e não só o chão. Respeita meshes ocultas da mesma forma que `Floor`.
- `Request`: contorno, `ZMin`, `ZMax`, quantidade, raio, afastamento, altura do monstro e semente.
- `Result`: pontos, quantos couberam, área de chão livre, espaçamento médio e menor espaçamento.
- **Algoritmo** (o de `docs/plan-populacao.md`, seção Distribuição):
  - grade de 16 unidades;
  - célula dentro do contorno a pelo menos 1 raio da borda;
  - chão: o primeiro terreno ou BSP com `n.z ≥ 0,65` descendo do `zmax` até o `zmin`, e nunca chão de mesh;
  - fora d'água: chão abaixo do topo de um volume de água que o contém descarta a célula;
  - obstáculo: triângulo de mesh ou face de BSP que não é chão, recortado na fatia `[chão − 16, chão + altura]`, a menos de raio + afastamento do centro da célula em XY, com descarte prévio por caixa expandida;
  - sorteio best-candidate de Mitchell com k = 20 e PCG de `math/rand/v2` com a semente, distância mínima de 2 raios entre pontos, posição dentro da célula e heading em 1..65535 tirados do mesmo gerador.
- **Determinismo:** a ordem de iteração não depende de mapas Go nem da ordem de carga dos tiles. A mesma `Geometry`, o mesmo `Request` e a mesma semente dão os mesmos pontos.
- A geração roda fora do laço de eventos, sobre um snapshot do `World`, como a cobertura vertical faz hoje. O resultado volta como um único `SetPoints`.

### D4. XML de spawn

- Módulo novo `spawnxml`, no molde de `zonexml`.
- Cabeçalho `<?xml version="1.0" encoding="utf-8"?>`, `<!DOCTYPE list SYSTEM "spawn.dtd">` e `<list>`, com indentação de tab e fim de linha `\n`.
- Um `<spawn name="[<nome>_<i>]">` por ponto, com `i` a partir de 0 na ordem dos pontos, e um `<npc id="…" count="1" respawn="…" [respawn_rand="…"] pos="x y z h" />`. `respawn_rand` só aparece quando é maior que 0.
- Áreas em ordem de ID e pontos na ordem do documento. Escape XML no nome.
- Nunca emite `<mesh>`, `event_name`, `period_of_day`, `respawn_cron` nem `ai_params`.
- A janela de XML existente mostra o arquivo e copia o texto, sem nenhum widget novo.

### D5. Projeto versão 2

- `Project` ganha `Spawns` e a versão passa a 2. Um arquivo da versão 1 abre com `Spawns` vazio. A versão 2 é recusada por binários com versão máxima 1, porque o `Load` de hoje já recusa versão maior que a atual.
- O `session` grava e restaura os 2 documentos. O indicador de alterações não salvas considera os 2.

### D6. Modelos embutidos

- Módulo novo `model`: `Decode([]byte) (*Bundle, error)` e `Encode(*Bundle) []byte` do formato `UE2HUM01` exatamente como o `bundle.rs` do UE2-Studio o define. Inclui a amostragem de pose (trilhas por frame, interpolação, composição da paleta), a transição entre clips e o skinning na CPU, que dá posições e normais em coordenadas do modelo.
- Módulo novo `skeletal`, usado só pela CLI: lê SkeletalMesh e MeshAnimation de pacotes 132/40, LOD 0, com a montagem das wedges do Lineage, até 4 influências por vértice e as trilhas de animação. É um port das partes de `skeletal.rs`, `skin.rs`, `meshanim.rs` e `pose.rs` do UE2-Studio que esse caminho exercita. Ler outras versões, gravar e PSK/PSA ficam de fora.
- **CLI `zbmodel`:** flags de cliente, mesh, animação, skins, clips, drawscale (padrão 1) e saída.
  - Resolve as skins pelo grafo de material de `internal/unreal`, decodifica para RGBA e grava como PNG.
  - Remove o root motion como o `Assets::load_client`.
  - Põe os pés em Z = 0 na pose do primeiro clip.
  - Imprime o raio (mediana da distância horizontal dos vértices, a regra do `npcgrp.rs`) e a altura da pose.
- **Monstro:** `LineageMonsters15.death_knight_wizard_m00`, `LineageMonsters15.death_knight_wizard_anim` e as skins `LineageMonstersTex9.death_knight_wizard.death_knight_wizard_t00` e `_t01`, com o clip `Wait`, em escala nativa.
- **Humano:** o `human.bin` do UE2-Studio, copiado sem alteração.
- Os 2 arquivos ficam embutidos com `//go:embed`, com um README de procedência. O raio e a altura do monstro viram constantes ao lado do embed.
- Nenhum arquivo do UE2-Studio é alterado e nenhum código Rust roda.

### D7. Render de modelos

- Os bindings GLES ganham `glDrawElementsInstanced` e `glVertexAttribDivisor`.
- Um passe novo, entre Masked e Translucent, desenha um modelo com N instâncias. O VBO de vértices sobe já com o skinning da CPU do frame. Cada instância passa `x y z yaw` por atributo com divisor 1, e o shader converte do servidor para a cena (`FromServer`, rebase).
- O shader é texturizado, com corte em alfa 0,5 nas seções masked e a luz fixa `0,60 + 0,40·max(N·L, 0)`. Depth test com Z reverso (ADR 0001).
- Todas as instâncias de um modelo usam a mesma pose no frame.
- O heading do L2 vira yaw na mesma unidade (65536 = 360°). Isso é **[INFERENCE]**, confirmado no teste em jogo.

### D8. Modo jogo

- Módulo novo `play`, puro: BVH de triângulos (divisão pela mediana, até 12 por folha), cápsula (raio 16, altura 80, olho a 64), gravidade 980, pulo de 360, corrida de 260 (×2 com Shift), voo de 900 sem colisão, sub-passos de até 4 unidades, resolução da penetração em até 6 iterações, chão quando `n.z > 0,65`, câmera com braço de 240 e varredura de esfera de raio 6. São as constantes do `play_map.rs` do UE2-Studio.
- **Colisão:** terreno, BSP sólido (sem a flag 0x8) e meshes que bloqueiam. `internal/unreal` passa a ler `bCollideActors`, `bBlockActors`, `bBlockPlayers`, `bWorldGeometry` e `bBlockNonZeroExtentTraces`, com o padrão da classe quando a propriedade não vem gravada, como faz o `play_collision` do UE2-Studio.
- **Sessão:** a BVH é preparada fora do laço de eventos, com o aviso de carga. A sessão guarda a câmera de edição e a restaura com Esc. O estado do editor do modo ativo não é tocado.
- **Humano:** idle, corrida, queda e pulo, escolhidos pelo estado da cápsula, com transição de 0,2 s e yaw pelo movimento.
- **Olhar:** por arraste do mouse, reaproveitando os controles de voo de hoje. A captura relativa do ponteiro fica fora, porque o Gio não tem esse recurso **[INFERENCE]**.

### D9. Idiomas

- Catálogo novo `spawn` em pt-BR e en, com as regras de `internal/locale`: chaves literais, paridade entre idiomas e variantes de plural.
- As contagens usam `locale.Plural`.

## Testing Decisions

Cada teste nomeia o bug concreto que pegaria. Só se testa comportamento externo do seam: nem a ordem interna do algoritmo, nem cor, nem texto de painel, nem o encaminhamento entre UI e documento.

### Seams

São 4 seams, todos no ponto mais alto possível. Os 3 primeiros são seams puros, no molde de `internal/water` e `internal/coverage`:

| Seam | Entrada | Saída observável | Prior art |
| --- | --- | --- | --- |
| `spawn.Document` | comandos | áreas, problemas, avisos, XML compilado | os testes de documento, problemas e compilação de `zone` |
| `placement.Distribute` | `Geometry` falsa (triângulos e água em coordenadas do servidor) + `Request` | pontos e números do `Result` | o `floor` falso dos testes de `coverage`, e `scenetest` para BSP e água |
| `model` | `human.bin` embutido e bundles sintéticos | bytes, vértices posados, limites | os testes de decodificação de `texture` |
| `play` | triângulos sintéticos + sequência de entradas | posição, velocidade e estado da cápsula | o seam sintético de `water` |

O projeto continua testado no seam que já existe, em `project`.

### Casos

- **`spawn.Document`:**
  - mudar o contorno, a faixa, a quantidade, o raio ou o afastamento depois de `SetPoints` marca a área como desatualizada e bloqueia `Compile`; mover, apagar ou adicionar ponto não marca. Pega a impressão errada ou ajuste manual tratado como mudança de entrada;
  - `respawn_rand > respawn`, id 0, nome repetido e área sem pontos bloqueiam, e os avisos de "cabem K de N" não bloqueiam. Pega um erro que o servidor transforma em perda do arquivo inteiro;
  - 1 `Undo` desfaz uma geração inteira. Pega pontos gravados em vários passos;
  - o XML compilado tem 1 `<spawn>` por ponto, `count="1"`, heading nunca 0, `respawn_rand` só quando é maior que 0 e nenhum `<mesh>`; compilar 2 vezes dá bytes idênticos. Pega mesh emitida, heading ignorado pelo servidor ou ordem instável.
- **`placement.Distribute`:**
  - um campo plano sem obstáculos e N pequeno gera N pontos dentro do contorno a pelo menos 1 raio da borda, com todos os pares a pelo menos 2 raios. Pega a borda ignorada ou sobreposição;
  - a mesma entrada com a mesma semente dá pontos iguais, e outra semente dá outros. Pega uma iteração não determinística;
  - uma caixa de mesh no meio do campo deixa livre só o que está a mais de raio + afastamento dela, e uma mesh logo fora do contorno bloqueia a faixa de dentro. Pega o afastamento contado do centro da mesh ou só dentro do contorno;
  - uma copa (triângulos acima da altura do monstro) não bloqueia, e um tronco que corta a fatia bloqueia. Pega a fatia ignorada;
  - chão de mesh nunca recebe ponto, e chão de terreno com `n.z < 0,65` também não. Pega a superfície ou o limite de inclinação trocados;
  - um volume de água cobrindo metade do campo deixa essa metade sem pontos. Pega a água ignorada;
  - uma ponte (chão de BSP acima do terreno) com a faixa no terreno põe os pontos no terreno, e com a faixa na ponte, na ponte. Pega a camada errada;
  - uma área pequena demais devolve os K que couberem e K < N. Pega espaçamento violado para caber N, ou laço infinito.
- **`model`:**
  - decodificar e recodificar o `human.bin` dá os mesmos bytes. Pega qualquer campo do formato lido ou gravado errado;
  - um buffer truncado ou com bytes sobrando é recusado. Pega a leitura parcial aceita;
  - o `human.bin` posado no frame 0 do idle tem o menor Z dos vértices em 0 (±0,5) e altura 80 (±1). Pega paleta ou inverse bind errados;
  - um bundle sintético de 2 ossos, com rotação conhecida no frame 1, leva um vértice ao ponto esperado, e a transição na metade fica entre as 2 poses. Pega a interpolação ou a ordem de composição.
- **`play`:**
  - a cápsula solta acima de um plano para em pé sobre ele, e a cápsula andando contra uma parede para nela. Pega a penetração não resolvida;
  - uma rampa de 30° é subida e uma de 60° não. Pega o limite de chão;
  - o pulo sobe e volta ao chão. Pega o lançamento para cima em contato com degrau (o clamp de velocidade);
  - o voo atravessa a parede. Pega colisão no voo;
  - o braço da câmera encurta atrás de uma parede.
- **Cliente real** (pulado sem `ZB_CLIENT`, como os testes de cena):
  - o `zbmodel` lê o `death_knight_wizard_m00`, e a pose `Wait` tem as 2 seções com textura, nenhuma rosa ou vazia, e o menor Z em 0. Pega skin não resolvida ou a montagem das wedges errada;
  - o mesmo tile do teste de cena, com uma área conhecida, gera pontos que nenhum triângulo de mesh do tile viola pela regra de obstáculo. Pega a `Geometry` do `World` em coordenadas erradas (cliente × servidor, rebase).
- **`project`:** um projeto com 2 áreas gerado, salvo e reaberto devolve áreas e pontos idênticos. Um arquivo da versão 1 abre sem áreas e com as zonas intactas. O teste de recusa de versão maior que a atual continua passando.
- **Idiomas:** os testes de catálogo de `locale` passam a cobrir o catálogo `spawn`.
- **Smoke no app** (registrado nos tickets, sem teste automático):
  - tela inicial → popular zona → área de 50 numa clareira com árvores, pedra e cercas → gerar → prévia → modo jogo → compilar → copiar;
  - modo de zonas → jogar dentro de uma zona → Esc, com a seleção e o desfazer intactos.
- **Servidor** (fora do repositório, no molde de `zone-builder-notes/zoneparser-harness`):
  - um harness carrega o XML de um projeto com 3 áreas pelo `SpawnParser` sem exceção;
  - em jogo, depois de reiniciar com o arquivo em `data/spawn/`, o `//pos` ao lado de 3 monstros fica a menos de 16 unidades dos pontos da prévia, e o heading bate.

## Out of Scope

- A saída `<mesh>` com sorteio do servidor. O app só emite pontos fixos.
- Ler o `Npcgrp.dat` ou qualquer `.dat` do cliente para mostrar o modelo de cada id: a prévia sempre usa o death knight wizard.
- Qualquer mudança no UE2-Studio.
- Ler o datapack: id, respawn e raio são digitados, e o app não confere se o id existe.
- Exclusões no spawn (o servidor não tem), `event_name`, `respawn_cron`, `period_of_day`, `ai_params` e rotas.
- Ler, importar ou editar os spawns que o datapack já tem.
- Ler geodata. Um ponto livre no cliente pode cair numa célula que a geodata bloqueia; com `pos` fixo, o servidor nasce o monstro ali mesmo assim.
- Gameplay no modo jogo: combate, IA, movimento dos monstros e colisão com eles.
- Animação do monstro além do idle, fases diferentes por instância e outros monstros de prévia.
- Captura relativa do ponteiro no modo jogo.
- Clientes 41x e qualquer outro formato de pacote de animação além do 132/40.

## Further Notes

- **Ordem dos tickets:** tela inicial e modos → documento de spawn e projeto v2 → distribuição → XML de spawn → (modelos e render instanciado) → prévia → modo jogo → docs e glossário → verificação em jogo. O módulo `model`, o `skeletal` e o `zbmodel` podem correr em paralelo com a distribuição, e o `play` em paralelo com a prévia.
- **Riscos:**

| Risco | Mitigação |
| --- | --- |
| O heading do L2 não é o yaw do Unreal na mesma unidade | Teste em jogo; se errar, uma conversão única no compilador e na prévia. |
| O monstro em escala 1 não bate com o tamanho no jogo (o drawscale do npcgrp não é lido) | Flag de drawscale no `zbmodel`; conferência ao lado do humano no modo jogo. |
| O cajado não vai para a mão na pose `Wait` | Conferência visual no critério da prévia; se for preciso, o `zbmodel` inclui o clip que o UE2-Studio usa na pose de referência. |
| Ponto livre no cliente, bloqueado na geodata | Fora do escopo; o servidor nasce o monstro mesmo assim, e a verificação em jogo mostra se é frequente. |
| O port do leitor de skeletal mesh diverge do UE2-Studio | A comparação visual com a captura do UE2-Studio e o teste do cliente real sobre as 2 skins. |
| Prévia lenta com muitas instâncias | Skinning uma vez por frame + 1 chamada instanciada; critério de 500 instâncias a 60 fps. |
