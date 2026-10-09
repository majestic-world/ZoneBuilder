# 14: Painel de propriedades

**What to build:** o usuário edita nome, tipo e parâmetros da zona selecionada num painel que só oferece valores que o servidor aceita.
- **Tipo:** lista fechada com os 23 valores do enum `ZoneType` do servidor, na caixa exata.
- **Parâmetros conhecidos do `ZoneTemplate`:** `enabled`, `default`, `target`, `affect_race`, `skill_name`, `damage_on_hp`, `blocked_actions` e os demais, com campos tipados e o valor padrão visível.
- **Parâmetros livres:** chave/valor, para os que os scripts leem (`residence`, `distribution_id`, `playerMinLevel` e outros).
- **Saída:** os parâmetros compilam como `<set name="..." val="..." />`, na ordem do documento.

**Blocked by:** 05 (Primeira zona de ponta a ponta)

**Status:** resolved

- [x] O tipo só aceita os 23 valores, e a caixa é preservada no XML
- [x] Um parâmetro conhecido mostra o valor padrão e só aceita valores do tipo dele
- [x] Parâmetros livres compilam como `<set>` com `val`, na ordem em que foram adicionados
- [x] Teste do seam Documento: `Siege` em vez de `SIEGE` não pode ser definido como tipo

## Comments

Resolvido na branch de integração `zone-builder`. Branch `zb/14-painel-propriedades`. Catálogo de 26 parâmetros do `ZoneTemplate` com tipo e padrão; todos os 2473 `<set>` do datapack aceitos. Teste `Siege` recusado. Manual: GameServer real.

Em 2026-10-08, por decisão do usuário, a lista tipada dos parâmetros conhecidos do `ZoneTemplate` (catálogo, validação por tipo e tabela de SystemMsg) foi removida: o foco do app são as coordenadas. Ficam o tipo e os parâmetros chave/valor, usados para `residence`, `distribution_id` e `fishing_place_type`.
