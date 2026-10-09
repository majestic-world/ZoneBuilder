# 06: Docs

**What to build:**

- **ADR 0006:** a regra de camadas com o terreno em bloco e o fallback; casos, alternativas descartadas (terreno peça a peça, que corta morros) e consequências.
- **ADR 0004:** o parágrafo Camadas aponta para o ADR 0006.
- **`GLOSSARY.md`:** "Outras camadas" passa a incluir o terreno; entram "Origem da faixa nova" e "Anel".
- **`README.md`**, nos trechos que hoje descrevem o comportamento antigo:
  - **"Regra das camadas"** (diz "O terreno sempre conta"): passa a ter o terreno em bloco e o fallback;
  - **item "Outras camadas"** dos números (diz "pisos de BSP ou mesh"): passa a incluir o terreno;
  - **parágrafo da criação** ("Ao criar um retângulo…"): ganha o seletor "Faixa nova" e a mensagem "pelos pontos clicados";
  - **linha "faces com listras fracas"** da tabela do viewport: passa a falar dos anéis do prisma oco, com a parte enterrada em anéis fracos e arestas tracejadas;
  - **pegada:** diz que ela só pinta o chão, não paredes nem tetos.

**Blocked by:** 05

**Status:** resolved

- [x] ADR 0006 escrito e citado pelo ADR 0004
- [x] O glossário não diz mais "O terreno nunca é outra camada"
- [x] O README não descreve mais o terreno que sempre conta, as faces listradas nem a pegada nas paredes, e descreve o seletor "Faixa nova"

## Comments

Implementado na branch `zo/06-docs` e integrado em `zona-oca-e-faixa-pelo-clique`. A branch do ticket só trouxe o `docs/adr/0006-terreno-como-camada.md` (`2f15ad1`). As edições do `GLOSSARY.md`, do `README.md` e do ADR 0004 ficaram no diretório da integração e entraram em `ebe00c0`.

O que mudou em cada documento:
- **ADR 0006:** registra o terreno julgado em bloco, com fallback, a tabela de casos com os números medidos no 23_18 e as alternativas descartadas. Nas consequências, registra a cascata dos pisos internos da torre.
- **ADR 0004:** o parágrafo Camadas agora aponta para o 0006 e mantém a regra do julgamento único.
- **Glossário:** "Outras camadas" passa a incluir o terreno, e entram os verbetes "Origem da faixa nova" e "Anel".
- **README:** tem a regra nova, o seletor "Faixa nova", os anéis do prisma oco e a pegada só no chão.
