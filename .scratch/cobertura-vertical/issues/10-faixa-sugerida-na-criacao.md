# 10: Faixa sugerida pela área na criação

**What to build:** retângulo, círculo, polígono fechado e tile inteiro já nascem com a faixa calculada pelo chão da área, com a mesma regra de camadas do ajuste. A prévia do retângulo e do círculo continua sendo exatamente o que entra no documento: ela pede a medição do contorno sob o cursor em segundo plano e mostra a faixa pelos vértices, marcada como "medindo…", até a medição chegar; o clique final mede na hora se ela ainda não chegou. Quando a área não tem chão medido (fora dos tiles carregados), a faixa sai pelos vértices, como hoje, e a mensagem diz isso (spec D5).

**Blocked by:** 09

**Status:** resolved

- [x] No app, um retângulo novo sobre o morro da captura nasce com cobertura de 100%
- [x] A faixa da prévia, depois de medida, é a mesma do shape criado pelo clique
- [x] Um polígono fechado sobre um vale entre morros nasce com o topo acima dos morros
- [x] Um shape criado parcialmente fora dos tiles carregados nasce com a faixa pelos vértices e a mensagem avisa

## Comments

Implementado na branch `cv/10-faixa-sugerida-na-criacao` e integrado em `cobertura-vertical`. Smoke: retângulo novo sobre o morro nasce com 100% dentro; a faixa da prévia é a mesma do shape criado pelo clique; polígono no vale nasce com o topo acima do morro; círculo que sai do tile carregado nasce pelos vértices, e a mensagem avisa. Seam: `TestAreaInBoxFollowsAConcaveOutline`.

Evidências: `C:/Workspace/zone-builder-notes/cobertura-vertical/10-evidence/`.
