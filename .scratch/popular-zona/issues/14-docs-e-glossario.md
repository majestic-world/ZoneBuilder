# 14: Docs e glossário da população

**What to build:** a documentação passa a descrever o modo população: o README explica a tela inicial, o modo população (área, gerar, ajustar pontos, prévia, modo jogo, compilar) e a flag `-mode`; o `GLOSSARY.md` ganha a seção "População" com os termos da spec (área de spawn, ponto de spawn, raio, afastamento, célula livre, semente, pontos desatualizados, monstro de prévia, modo jogo); o `docs/plan-populacao.md` marca os marcos P0–P5 com o resultado; os READMEs de procedência dos 2 modelos citam o comando de regeneração; e um ADR registra a saída por ponto fixo (`pos`) em vez de `<mesh>`, com o resultado da verificação em jogo.

**Blocked by:** 12, 13

**Status:** ready-for-agent

- [ ] O README descreve a tela inicial, o modo população e o modo jogo nos 2 modos
- [ ] O `GLOSSARY.md` tem a seção "População" com os termos da spec
- [ ] O ADR registra a decisão de ponto fixo e o resultado da verificação do ticket 13
- [ ] O plano de população reflete o que foi entregue
