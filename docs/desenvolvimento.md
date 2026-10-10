# Desenvolvimento

[Apresentação do projeto](../README.md) · [Guia de uso](uso.md)

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
make dist    # compila e gera "dist/Zone Builder By Mk v<versão>.zip" com o conteúdo de bin/, apagando os zips antigos de dist/
```

A versão exibida no título da janela vem de `APP_VERSION`, no arquivo `.env`.

O executável é um app de interface do Windows (`-H windowsgui`): aberto pelo Explorer ou por atalho, não abre janela de terminal. Iniciado de um terminal (inclusive por `make run`), o log aparece nesse terminal.

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

Os testes que leem o cliente real (em `internal/l2pkg`, `cmd/zbmodel` e outros) usam a pasta indicada por `ZB_CLIENT`; sem a variável, esses testes são pulados:

```sh
ZB_CLIENT="C:/caminho/do/cliente" go test ./...
```

## Traduções para desenvolvimento

Mensagens da interface ficam embutidas em `internal/locale/catalog/<área>/pt-BR.json` e `en.json`. Ao acrescentar uma área, crie **os dois arquivos**: cada chave estável começa com `<área>.`, e os dois idiomas precisam conter as mesmas chaves, os mesmos parâmetros nomeados (`{name}`) e, para mensagens com contagem, as variantes `one` e `other`. A forma `one` vale somente para 1; `other` vale para 0 e demais contagens. Use uma frase completa por variante, não fragmentos concatenados.

Apresente texto estático com `locale.Text`, texto parametrizado com `locale.Format` e contagem com `locale.Plural`. Números exibidos usam `locale.Number` ou `locale.Percent` antes da interpolação; coordenadas, valores digitados, projetos e XML permanecem literais. Para status que sobrevivem à troca de idioma, retenha `locale.Message` com chave e dados; submensagens em `Parts` são formatadas no idioma corrente ao chamar `Render`. **Todas as chaves usadas em chamadas e mensagens precisam ser literais**, inclusive nos mapas de `Parts`: `go test ./internal/locale` rejeita chaves dinâmicas e confere automaticamente pares, parâmetros, variantes, duplicatas e usos sem tradução, sem lista manual de áreas.

Logs, comentários de código, documentação do repositório e ferramentas de diagnóstico para desenvolvedores (como `zbdump`) ficam em português e não passam pelos catálogos; ver [ADR 0009](adr/0009-interface-bilingue-pt-br-en.md).

Nenhuma tradução pode depender de larguras dimensionadas para a frase em português: confira cada texto novo nos dois idiomas na janela padrão de 1280×800, sem sobrepor inspetor, barra ou viewport.

## Verificação manual

Cenários de fumaça e regressão para repetir no app depois de mudanças nas áreas abaixo, com os valores de referência medidos no cliente Fafurion. Verificações pendentes no servidor ficam nos documentos de cada decisão: [ADR 0004](adr/0004-chao-medido-e-avisos-sem-bloqueio.md), [ADR 0005](adr/0005-agua-30-abaixo-do-volume.md) e [ADR 0007](adr/0007-ponto-fixo-em-vez-de-mesh.md).

### Idiomas

1. Com uma configuração isolada e sem preferência, o app abre em PT-BR. Com a preferência `en`, abre em EN.
2. Abra um projeto com uma zona que tem problema bloqueante e outra com aviso de cobertura. Clique em cada linha e confira que cada uma leva à sua zona. Compile só a zona válida: o XML sai apesar do aviso.
3. Com a janela de alturas e a de XML abertas, role o inspetor e foque um campo (margem Z com `512`). Troque PT-BR → EN: títulos, causas de problema, medidas, status de cópia e aviso mudam de idioma; conteúdo dos campos, foco, seleção, janelas e rolagem ficam como estavam; o XML copiado antes e depois é idêntico byte a byte (referência: 22_22, 267 bytes).
4. Abra **Abrir projeto**, **Salvar projeto** e a escolha da pasta do cliente: o texto fornecido pelo app está em EN (o resto segue o idioma do Windows). Cancele.
5. Com vizinhos 3×3 ligados, carregue 25_25 e troque o idioma no meio da carga: o texto de progresso muda (`Loading: 0 of 9 tiles ready`) sem reiniciar a carga, os 9 tiles carregam e as zonas são mantidas.
6. Feche e reabra: o app volta em EN. Volte para PT-BR.
7. Em 1280×800, nenhum texto em inglês sobrepõe inspetor, barra ou viewport.

### Torre 23_18 (prisma oco e faixa pelo clique)

Tile 23_18 (Tower of Insolence; topo do BSP em z ≈ 10083–10197).

1. Visão de dentro: `-camera 114750,16125,10350,…`. O mapa aparece limpo, com anéis nas paredes e a pegada só no chão (antes, a tela inteira ficava verde e hachurada).
2. **Chão da área**: polígono de 6 vértices no topo (BSP z 10083) nasce com `8464..10340`; o terreno (−5120) é o primeiro em "Outras camadas".
3. **Pontos clicados**: polígono de 4 vértices em z 10084 → `9828..10340`; de 6 vértices em z 10083 → `9827..10340`; retângulo → 9827…10339, prévia igual ao clique; círculo → 9827…10603; "medindo…" nunca aparece.
4. Tile inteiro com o seletor em Pontos clicados: `−5632..396 pelo chão da área` (o seletor é ignorado).
5. Sem regressão no 22_22: retângulo no morro em Chão da área → −4004…−984.
6. Zona legada (`−5376…10453`): Base 9941, Altura 512 e depois "Piso ao chão" → 8512…10453.

Lacunas conhecidas: a cintilação dos anéis numa parede vista quase de lado só foi conferida em quadros parados, não com a câmera em movimento; a prévia de desenho (retângulo, círculo, polígono fechado) não foi conferida à parte no estilo de anéis, pois passa pelo mesmo `zoneOverlay.set()`.

### Água

1. 22_22 (mar de Giran): clique num volume e Ctrl+clique nos outros 7. Status: `Água: 8 volumes · topo -3780 (servidor -3810) · exata`. Clique direito → **Compilar zona de água**: `zonebuilder_water.xml` tem 1 zona `[22_22_WaterVolume0]` com 8 polígonos. A zona sobrevive a salvar e reabrir, e Ctrl+Z a remove. Compilar de novo não cria nada e mostra `Zona [22_22_WaterVolume0] já existe no projeto: compilada a versão do projeto`.
2. 22_24 (Floran): os 11 volumes de topo −3780 dão 1 zona com 11 polígonos, com pontos e `zmin`/`zmax` iguais aos de `[22_24_water1…9]`, `[22_24_water12]` e `[22_24_water13]` do datapack. A fonte, `WaterVolume26` (topo −3746) e `WaterVolume27` (topo −5529), que se tocam, dá 2 zonas `[22_24_WaterVolume26]` e `[22_24_WaterVolume27]` com zmax −3776 e −5559.
3. 25_25: compilar só `WaterVolume9` (topo −3788) avisa `Água sobreposta` com `WaterVolume7` (topo −3020), `WaterVolume11` e `WaterVolume4`.
4. Com um polígono aberto, o item do menu fica desabilitado.

### Popular zona

1. Tela inicial → **Popular zona** → área de 50 pontos numa clareira com árvores, uma rocha e cercas → Gerar → Prévia → Jogar → Compilar → Copiar: a área de transferência é idêntica ao XML.
2. Modo de zonas → Jogar dentro de uma zona → Esc: seleção e histórico de desfazer inalterados, câmera restaurada.
3. Prévia com 500 instâncias e `-fps`.
4. Trocar o conjunto de tiles durante o jogo reconstrói a árvore de colisão (conferido com 6, 7, 8 e 9 tiles); abrir outro mapa encerra a sessão.

## Licenças de terceiros

- ANGLE (`third_party/angle`): BSD.
- Ícones Lucide (`internal/ui/icon/lucide`): ISC.
- Fonte Inter (`internal/ui/fonts`): SIL Open Font License.
