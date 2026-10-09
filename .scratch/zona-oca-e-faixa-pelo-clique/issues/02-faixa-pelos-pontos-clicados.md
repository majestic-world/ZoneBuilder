# 02: Faixa nova pelos pontos clicados

**What to build:** na seção Edição do inspetor, abaixo de "Folga Z da faixa sugerida", um seletor "Faixa nova", com setas, no padrão do seletor de Tipo: **Chão da área** (padrão) | **Pontos clicados**. Ele vale para polígono, retângulo e círculo; o tile inteiro sempre usa o chão da área (spec D2). No modo pontos clicados, `zmin zmax` é `zone.SuggestZRange(vértices, folga)`, sem medir o perfil, na prévia e no clique, e a mensagem diz "pelos pontos clicados" (nova `zSource` `zByVertices` e chave `editor.source.vertices` em pt-BR e en). A escolha vale só na sessão, como a folga.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] No app, no 23_18, no modo pontos clicados: o polígono no topo da torre nasce com `menorZ − 256 … maiorZ + 256` e a mensagem "pelos pontos clicados"
- [x] Retângulo e círculo no topo da torre, no modo pontos clicados: a prévia mostra a mesma faixa que o clique cria, sem "medindo…"
- [x] No modo chão da área, o comportamento é o do ticket 01 (sem regressão no retângulo sobre o morro do 22_22)
- [x] O tile inteiro ignora o seletor
- [x] Rótulos do seletor nos 2 idiomas, passando no `locale` check

## Comments

Implementado na branch `zo/02-faixa-pelos-pontos-clicados` e integrado em `zona-oca-e-faixa-pelo-clique`. O seletor fica em `EditPanel.FromVertices`, com as setas `PrevFrom`/`NextFrom`. O `floorCoverage.suggest` devolve `zByVertices` antes de medir o perfil, a não ser no tile inteiro (`fromTerrain`).

Smoke no 23_18:
- Polígono de 4 vértices a z 10084: `z 9828..10340 pelos pontos clicados`.
- Retângulo: prévia e clique em 9827 … 10339.
- Círculo: prévia e clique em 9827 … 10603.
- Em nenhum dos 3 apareceu "medindo…".
- Tile inteiro com o seletor em pontos clicados: `z -5632..396 pelo chão da área`.
- Retângulo sobre o morro do 22_22 em chão da área: −4004 … −984, sem regressão.

As chaves `ui.edit.new_range*` e `editor.source.vertices` existem em pt-BR e en, e `TestBundledCatalogsCoverLiteralCodeUses` passa.

Limitação conhecida: com o retângulo ou o círculo já ancorado, a prévia só se atualiza depois de trocar o seletor e mover o cursor.

Evidências: `C:/Workspace/zone-builder-notes/zona-oca-e-faixa-pelo-clique/02-evidence/`.
