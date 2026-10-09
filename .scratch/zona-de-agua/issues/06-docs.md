# 06: Docs

**What to build:** documentar a zona de água onde o usuário e o próximo agente vão procurar:

- `GLOSSARY.md`: seção "Água" com volume de água, corpo d'água, topo e exata/aproximada, com os termos a evitar (superfície de água não é volume; "lago" não é termo do código).
- `README.md`: o fluxo clique → botão direito → "Compilar zona de água", o nome `[X_Y_<volume>]`, e o aviso de que uma zona de água antiga do datapack no mesmo lugar precisa ser removida, porque o servidor usa o maior `zmax` das duas.
- `docs/plan.md`: a leitura de `WaterVolume` (brush, faces BSP, transformação ABrush) em "Fatos que moldam o design" e o `internal/water` no fluxo.
- Spec: `Status: resolved` com o resumo.

**Blocked by:** 04 (Menu "Compilar zona de água"); o 05 fica aberto para o usuário (decisão 4 da spec: o −30 sai marcado como pendente da medição em jogo)

**Status:** ready-for-agent

- [ ] Os 3 arquivos atualizados, com os números do ADR 0005 já confirmados
- [ ] Nenhuma menção a um comportamento que o ticket 05 mudou

## Comments
