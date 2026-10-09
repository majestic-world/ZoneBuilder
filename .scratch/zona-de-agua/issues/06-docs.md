# 06: Docs

**What to build:** documentar a zona de água onde o usuário e o próximo agente vão procurar:

- `GLOSSARY.md`: seção "Água" com volume de água, corpo d'água, topo e exata/aproximada, com os termos a evitar (superfície de água não é volume; "lago" não é termo do código).
- `README.md`: o fluxo clique → botão direito → "Compilar zona de água", o nome `[X_Y_<volume>]`, e o aviso de que uma zona de água antiga do datapack no mesmo lugar precisa ser removida, porque o servidor usa o maior `zmax` das duas.
- `docs/plan.md`: a leitura de `WaterVolume` (brush, faces BSP, transformação ABrush) em "Fatos que moldam o design" e o `internal/water` no fluxo.
- Spec: `Status: resolved` com o resumo.

**Blocked by:** 04 (Menu "Compilar zona de água"); o 05 fica aberto para o usuário (decisão 4 da spec: o −30 sai marcado como pendente da medição em jogo)

**Status:** resolved

- [x] Os 3 arquivos atualizados, com o −30 do ADR 0005 marcado como pendente da medição em jogo (decisão 4 da spec)
- [x] Nenhuma menção a um comportamento que o ticket 05 mudou

## Comments

Implementado na branch `za/06-docs` e integrado em `zona-de-agua`: seção Água no `GLOSSARY.md`, seção Zona de água no `README.md`, fato do `WaterVolume` e `internal/water` no `docs/plan.md`, e a seção Resolução da spec. Se o ticket 05 trocar o offset, atualizar o README, o "Topo" do glossário, o fato do plano e a Resolução da spec.
