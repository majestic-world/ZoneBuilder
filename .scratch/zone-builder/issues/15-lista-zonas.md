# 15: Lista de zonas

**What to build:** o usuário vê e gerencia todas as zonas do projeto.
- **Lista:** nome, tipo e quantidade de problemas de cada zona.
- **Encontrar:** busca por nome e filtro por tipo.
- **Visibilidade:** mostrar ou ocultar zonas, uma a uma e por tipo.
- **Cor por tipo** no viewport.
- **Gerenciar:** renomear, apagar e duplicar.
- **Navegar:** selecionar uma zona leva a câmera até ela, e digitar uma coordenada `x y z` leva a câmera até o ponto.

**Blocked by:** 05 (Primeira zona de ponta a ponta)

**Status:** resolved

- [x] Com 3 zonas de tipos diferentes, a busca e o filtro por tipo reduzem a lista como esperado
- [x] Ocultar uma zona ou um tipo tira o prisma do viewport, e mostrar devolve
- [x] Selecionar uma zona na lista leva a câmera até ela
- [x] Digitar `x y z` leva a câmera até o ponto
- [x] Duplicar uma zona cria uma cópia com nome novo, e as 2 compilam de forma independente

## Comments

Resolvido na branch de integração `zone-builder`. Branch `zb/15-lista-zonas`. Lista com busca, filtro, ocultar, cor por tipo, renomear, apagar e duplicar; câmera vai até a zona e até `x y z`. Duplicata compilada e carregada de forma independente no harness.
