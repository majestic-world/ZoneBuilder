# 05: Exclusões na medição

**What to build:** o chão sob um shape banido da mesma zona, e dentro da faixa Z dele, deixa de contar como falha e aparece numa área própria, "excluída". A regra usa o centróide de cada peça medida, então o erro fica limitado a 1 célula ao longo do contorno da exclusão (spec D1).

**Blocked by:** 01

**Status:** resolved

- [x] Teste do seam: uma exclusão no meio de um shape põe o chão sob ela em "excluída", não em "acima do topo"
- [x] Teste do seam: uma exclusão cuja faixa Z não alcança o chão não exclui nada
- [x] Teste do seam: dentro + acima + abaixo + excluída continua igual à área total de chão medida
- [x] No app, o inspetor de um shape com uma exclusão mostra a área excluída

## Comments

Implementado na branch `cv/05-exclusoes-na-medicao` e integrado em `cobertura-vertical`. Seam: exclusão no meio conta como excluída, exclusão fora da faixa Z não exclui nada, e as 4 áreas somam o chão. Smoke: o inspetor mostra "Excluída: 19,7%". Depois da revisão, o chão excluído também deixou de definir extremos, folgas, pinos e ajustes.

Evidências: `C:/Workspace/zone-builder-notes/cobertura-vertical/05-evidence/`.
