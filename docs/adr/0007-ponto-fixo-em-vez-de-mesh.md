# Spawn por ponto fixo (`pos`), sem `<mesh>`

O modo população distribui os monstros pelo chão livre de uma área, longe das static meshes, e mostra cada um na prévia. O XML de spawn do servidor Java aceita 2 formas: um `<npc>` com `count` depois de uma `<mesh>`, ou um `<npc>` com `pos="x y z h"`. Com `<mesh>`, `SpawnMesh.getRandomLoc` sorteia um ponto novo no polígono a cada spawn e respawn, sem raio de colisão nem espaçamento, e só desvia das meshes que a geodata registra. A distribuição aprovada na prévia seria descartada na primeira subida do servidor.

## Decisão: 1 `<spawn>` por área, com `pos` por ponto

Cada área aprovada vira um `<spawn name="[<área>_0]">` contendo 1 `<npc id="…" count="1" respawn="60" pos="x y z h" />` por ponto. Este agrupamento substitui a saída anterior de 1 `<spawn>` por ponto, conforme pedido do usuário, sem alterar a decisão de posições fixas. O app não oferece a saída `<mesh>`, nem `event_name`, `period_of_day`, `respawn_cron` ou `ai_params`.

- `x y z` em coordenadas do servidor (Z do cliente + 32, [ADR 0003](0003-z-do-servidor-32-acima-do-cliente.md)). O servidor reajusta o Z pela geodata em `spawnMe0`; o Z do arquivo só escolhe a camada.
- O heading vai em 1..65535 (`spawnxml.Heading`): `Creature.spawnMe` só aplica `h > 0`, então 0 vira 1.
- `respawn` é fixo em 60 s; `respawn_rand` não é emitido. IDs positivos são escolhidos ao compilar, separados por espaço, e repartidos ciclicamente entre os pontos de cada área. A diferença de quantidade entre IDs distintos é de no máximo 1; repetições na entrada são ignoradas.
- Um arquivo por projeto, com nome livre (padrão: o nome do projeto), porque o `SpawnParser` varre `data/spawn/` e ignora o nome.

**Consequência aceita:** o monstro renasce sempre no mesmo ponto e vagueia até `MaxDriftRange` (200) em volta dele.

**Descartada:** `<mesh>` + `count`. Gera um XML menor, mas o servidor não respeita os pontos, o afastamento das meshes nem o espaçamento que a prévia mostra.

## Heading e yaw

A prévia gira o monstro por `yaw = h · 2π / 65536`, a partir de +X e no sentido de +Y. No código do servidor (só leitura):

- `SpawnParser` lê o 4º número de `pos` sem conversão, e `NpcInfo` manda esse `h` cru para o cliente;
- `PositionUtils.calculateHeadingFrom` é `atan2(dy, dx)` em graus × 182,044 (65536 = 360°, 0 = +X, crescendo para +Y), e o servidor usa esse valor para virar a criatura para o alvo e para o destino do movimento.

Mesma unidade, mesmo zero e mesmo sentido. Resta uma **[INFERENCE]** só do lado do cliente: que ele desenha o NPC com `Rotation.Yaw = h` e que a frente da mesh embutida é o mesmo +X.

## Verificação

**Histórico:** a saída anterior de 1 grupo por ponto, com 3 áreas (`zb_oeste`, `zb_centro`, `zb_leste`, 10 pontos na clareira do 22_22), foi compilada no app e carregada pelo harness do `SpawnParser` com os templates de NPC do datapack: 10 spawns carregados, nenhum problema. Um XML com `respawn_rand > respawn` falhou no mesmo harness, como esperado.

**Formato atual:** build, vet e suíte passaram. O smoke compilou e copiou o XML de 2 áreas com 50 e 5 pontos e IDs `1 2 3 4`: grupos únicos, posições/headings preservados, respawn 60 sem rand, divisões 13/13/12/12 e 2/1/1/1, e 2 compilações byte a byte iguais. Essa verificação não reiniciou o servidor nem substitui o teste em jogo; o roteiro abaixo usa o artefato anterior.

**Pendente:** a verificação em jogo, que depende do usuário. Com `tres-areas.xml` em `gameserver/data/spawn/` e o servidor reiniciado (de preferência com `RndWalk = False` em `config/ai.properties`, porque o monstro vagueia e gira quando um jogador chega perto):

| Spawn | `pos` | Heading | Ponto 150 à frente |
| --- | --- | --- | --- |
| `[zb_oeste_0]`, Gremlin 20001 | 86286 162022 −3576 16803 | 92,3° (≈ +Y) | 86280 162172 −3576 |
| `[zb_centro_2]`, Wolf 20120 | 86474 162331 −3580 56641 | 311,1° | 86573 162218 −3580 |
| `[zb_leste_1]`, Elpy 20432 | 86907 161898 −3563 34471 | 189,4° (≈ −X) | 86759 161874 −3563 |

Para cada spawn: `//teleport` ao ponto à frente, conferir que o monstro mostra o rosto, andar até ele e dar `//pos` encostado (o `//pos` é do jogador, não do alvo). Passa com `x y` a menos de 16 do `pos`, mais o raio de colisão do monstro. Quando a verificação for feita, esta seção registra:

- a diferença entre o `//pos` e o `pos` de cada um dos 3 spawns;
- se cada monstro olha para o ponto à frente. De costas (180°) ou de lado (±90°) quer dizer que o cliente usa outra convenção: a conversão entra num lugar só, no caminho da compilação junto com `headingYaw` da prévia;
- se a altura do monstro de prévia (66, 0,83 do humano de 80) parece a do jogo.

## Consequences

- A prévia é o que o servidor faz: posição, camada e heading saem do arquivo, não de um sorteio do servidor.
- O app não lê a geodata: um ponto livre no cliente pode cair numa célula que a geodata bloqueia, e com `pos` o monstro nasce ali mesmo assim.
- Mudar o contorno, a faixa Z, a quantidade, o raio ou o afastamento depois de gerar deixa os pontos desatualizados, e isso bloqueia a compilação até gerar de novo. Mover, apagar e adicionar pontos à mão não deixa a área desatualizada.
- Um arquivo com muitos pontos tem 1 `<npc>` por monstro dentro do grupo da área; o tamanho cresce linearmente com a quantidade.
