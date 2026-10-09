# 07: Linha do chão nas paredes do prisma

**What to build:** em cada parede do prisma do shape selecionado, uma linha mostra o chão ao longo da aresta, na altura exata dele: os pontos são os cruzamentos da aresta com os triângulos do chão (spec D4d). Junto com o tracejado da parte enterrada, dá para ler em cada parede quanto do prisma está acima e quanto está abaixo do chão. A linha acompanha a medição: some enquanto o contorno é arrastado e volta quando a nova medição chega.

**Blocked by:** 01, 03

**Status:** ready-for-agent

- [ ] Teste do seam: para uma aresta sobre uma rampa sintética, a linha do chão bate com a rampa nos cruzamentos de célula
- [ ] Captura de tela da zona da captura: em cada parede, a linha do chão acompanha o relevo e separa a parte sólida da tracejada
