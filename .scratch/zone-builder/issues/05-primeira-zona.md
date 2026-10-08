# 05: Primeira zona de ponta a ponta

**What to build:** o usuário cria uma zona com nome e tipo, clica no terreno para adicionar os vértices de um polígono e fecha com Enter ou clicando no primeiro vértice. A zona aparece como prisma translúcido entre `zmin` e `zmax`, com arestas e vértices desenhados por cima da cena. A faixa Z vem sugerida pelo menor e maior Z dos vértices com folga de 256. Ao compilar, sai 1 arquivo XML na pasta de saída escolhida.

O arquivo segue o formato que o `ZoneParser` aceita:
- cabeçalho `<?xml version='1.0' encoding='utf-8'?>`, `<!DOCTYPE list SYSTEM "zone.dtd">` e `<list>`;
- indentação com tab;
- coords de 4 números com a mesma faixa Z em todos os vértices.

Este ticket abre o seam Documento: `Apply(comando)` e `Compile`. A UI só produz comandos e nunca altera a zona direto.

**Blocked by:** 04 (Clique no terreno devolve `x y z`)

**Status:** resolved

- [x] Criar zona, clicar vértices, fechar o polígono e compilar gera um XML que o servidor Java carrega sem alteração e sem `invalid territory data`
- [x] O prisma aparece entre `zmin` e `zmax` sugeridos, com folga de 256 abaixo do menor Z e acima do maior
- [x] Teste do seam Documento: num polígono compilado, todas as coords trazem o mesmo `zmin zmax` de 4 números
- [x] Teste do seam Documento: compilar 2 vezes o mesmo documento gera bytes idênticos

## Comments

Resolvido na branch de integração `zone-builder`. Branch `zb/05-primeira-zona`. XML compilado carregado pelo `ZoneParser` sem alteração via harness offline (`LOADED`, exit 0). Testes de faixa Z e determinismo em `internal/zone`. Manual: carregar no GameServer real (precisa de banco).
