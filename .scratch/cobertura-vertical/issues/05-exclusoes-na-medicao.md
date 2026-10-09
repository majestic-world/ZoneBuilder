# 05: Exclusões na medição

**What to build:** o chão sob um shape banido da mesma zona, e dentro da faixa Z dele, deixa de contar como falha e aparece numa área própria, "excluída". A regra usa o centróide de cada peça medida, então o erro fica limitado a 1 célula ao longo do contorno da exclusão (spec D1).

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] Teste do seam: uma exclusão no meio de um shape põe o chão sob ela em "excluída", não em "acima do topo"
- [ ] Teste do seam: uma exclusão cuja faixa Z não alcança o chão não exclui nada
- [ ] Teste do seam: dentro + acima + abaixo + excluída continua igual à área total de chão medida
- [ ] No app, o inspetor de um shape com uma exclusão mostra a área excluída
