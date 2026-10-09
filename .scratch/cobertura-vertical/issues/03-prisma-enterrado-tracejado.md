# 03: Prisma enterrado tracejado

**What to build:** a parte do prisma que fica atrás do terreno deixa de sumir (faces) ou de ser desenhada por cima do terreno como se estivesse visível (arestas). As faces enterradas são desenhadas atrás da cena com alfa baixo e tracejado em espaço de tela, e as arestas ficam sólidas onde estão visíveis e tracejadas onde ficam atrás do chão. Assim o piso enterrado aparece tracejado sob o morro, e não por cima dele (spec D4d). Os handles dos vértices continuam desenhados por cima de tudo.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] Captura de tela da zona da captura: a aresta do piso aparece tracejada onde passa sob o terreno e sólida onde está à vista
- [x] As faces enterradas aparecem através do terreno, mais fracas que as visíveis
- [x] Os handles dos vértices continuam clicáveis e visíveis em qualquer ângulo
- [x] O log de `-fps` não piora de forma perceptível com uma zona selecionada

## Comments

Implementado na branch `cv/03-prisma-enterrado-tracejado` e integrado em `cobertura-vertical`. Smoke com capturas antes e depois: o piso enterrado aparece tracejado sob o morro e as faces enterradas aparecem mais fracas. FPS com zona selecionada: 83,6–87,3 antes, 82,8–86,8 depois. O clique e o arrasto de um handle com o prisma enterrado foram conferidos no smoke final.

Evidências: `C:/Workspace/zone-builder-notes/cobertura-vertical/03-evidence/, C:/Workspace/zone-builder-notes/cobertura-vertical/final-evidence/`.
