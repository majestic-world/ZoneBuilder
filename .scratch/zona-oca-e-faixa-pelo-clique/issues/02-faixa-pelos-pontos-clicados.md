# 02: Faixa nova pelos pontos clicados

**What to build:** na seção Edição do inspetor, abaixo de "Folga Z da faixa sugerida", um seletor "Faixa nova", com setas, no padrão do seletor de Tipo: **Chão da área** (padrão) | **Pontos clicados**. Ele vale para polígono, retângulo e círculo; o tile inteiro sempre usa o chão da área (spec D2). No modo pontos clicados, `zmin zmax` é `zone.SuggestZRange(vértices, folga)`, sem medir o perfil, na prévia e no clique, e a mensagem diz "pelos pontos clicados" (nova `zSource` `zByVertices` e chave `editor.source.vertices` em pt-BR e en). A escolha vale só na sessão, como a folga.

**Blocked by:** None (can start immediately)

**Status:** needs-triage

- [ ] No app, no 23_18, no modo pontos clicados: o polígono no topo da torre nasce com `menorZ − 256 … maiorZ + 256` e a mensagem "pelos pontos clicados"
- [ ] Retângulo e círculo no topo da torre, no modo pontos clicados: a prévia mostra a mesma faixa que o clique cria, sem "medindo…"
- [ ] No modo chão da área, o comportamento é o do ticket 01 (sem regressão no retângulo sobre o morro do 22_22)
- [ ] O tile inteiro ignora o seletor
- [ ] Rótulos do seletor nos 2 idiomas, passando no `locale` check

## Comments
