# 03: Prisma enterrado tracejado

**What to build:** a parte do prisma que fica atrás do terreno deixa de sumir (faces) ou de ser desenhada por cima do terreno como se estivesse visível (arestas). As faces enterradas são desenhadas atrás da cena com alfa baixo e tracejado em espaço de tela, e as arestas ficam sólidas onde estão visíveis e tracejadas onde ficam atrás do chão. Assim o piso enterrado aparece tracejado sob o morro, e não por cima dele (spec D4d). Os handles dos vértices continuam desenhados por cima de tudo.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] Captura de tela da zona da captura: a aresta do piso aparece tracejada onde passa sob o terreno e sólida onde está à vista
- [ ] As faces enterradas aparecem através do terreno, mais fracas que as visíveis
- [ ] Os handles dos vértices continuam clicáveis e visíveis em qualquer ângulo
- [ ] O log de `-fps` não piora de forma perceptível com uma zona selecionada
