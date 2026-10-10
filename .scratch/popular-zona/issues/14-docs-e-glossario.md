# 14: Docs e glossário da população

**What to build:** a documentação passa a descrever o modo população: o README explica a tela inicial, o modo população (área, gerar, ajustar pontos, prévia, modo jogo, compilar) e a flag `-mode`; o `GLOSSARY.md` ganha a seção "População" com os termos da spec (área de spawn, ponto de spawn, raio, afastamento, célula livre, semente, pontos desatualizados, monstro de prévia, modo jogo); o `docs/plan-populacao.md` marca os marcos P0–P5 com o resultado; os READMEs de procedência dos 2 modelos citam o comando de regeneração; e um ADR registra a saída por ponto fixo (`pos`) em vez de `<mesh>`, com o resultado da verificação em jogo.

**Blocked by:** 12, 13

**Status:** ready-for-human

- [x] O README descreve a tela inicial, o modo população e o modo jogo nos 2 modos
- [x] O `GLOSSARY.md` tem a seção "População" com os termos da spec
- [ ] O ADR registra a decisão de ponto fixo e o resultado da verificação do ticket 13
- [x] O plano de população reflete o que foi entregue

## Comments

Documentação integrada em `popular-zona`, a partir de `pz/14-docs-e-glossario` (`f002ee3`) e do complemento `24f14f1`: README, glossário, estrutura real do código, resultados P0–P5 e ADR 0007 de ponto fixo. A procedência dos modelos já inclui os comandos de regeneração.

O ADR registra o harness sem exceção e deixa explícita a pendência em jogo. A conclusão da verificação do ticket 13 ainda não existe; depois dela, preencher o resultado no ADR 0007 e no plano P3, marcar o critério restante e resolver este ticket. Notas: `C:/Workspace/zone-builder-notes/popular-zona/14-docs-e-glossario.md`.
