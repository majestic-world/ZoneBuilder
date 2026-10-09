# 14: Docs da cobertura vertical

**What to build:** a documentação passa a descrever a cobertura vertical: o README explica os números, as cores do chão, o tracejado, os pinos, a régua e os avisos; o plano do projeto ganha o item em M6 e M7; um ADR novo registra a definição de chão (terreno inteiro mais faces de BSP e mesh voltadas para cima, medidas de forma exata sobre os triângulos) e a decisão de avisar em vez de bloquear; e o glossário do projeto, que ainda não existe, nasce com os termos da cobertura (chão, camada, perfil do chão, folga do piso, folga do topo, cobertura, sem chão, pior ponto).

**Blocked by:** 02, 07, 10, 12, 13

**Status:** ready-for-agent

- [x] O README descreve o que cada cor e cada número significam
- [ ] O ADR 0004 registra as 2 decisões, com o resultado da verificação em jogo
- [x] O glossário existe na raiz com os termos da cobertura

## Comments

README, `docs/plan.md`, `docs/adr/0004-chao-medido-e-avisos-sem-bloqueio.md` e `GLOSSARY.md` estão escritos na branch `cv/14-docs`, já integrada em `cobertura-vertical`. Falta o resultado do ticket 13: a seção "Verificação em jogo" do ADR 0004 está marcada como pendente. Quando o 13 for resolvido, preencher essa seção e marcar o item 2.
