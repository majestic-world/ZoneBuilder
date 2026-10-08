# 14: Painel de propriedades

**What to build:** o usuário edita nome, tipo e parâmetros da zona selecionada num painel que só oferece valores que o servidor aceita.
- **Tipo:** lista fechada com os 23 valores do enum `ZoneType` do servidor, na caixa exata.
- **Parâmetros conhecidos do `ZoneTemplate`:** `enabled`, `default`, `target`, `affect_race`, `skill_name`, `damage_on_hp`, `blocked_actions` e os demais, com campos tipados e o valor padrão visível.
- **Parâmetros livres:** chave/valor, para os que os scripts leem (`residence`, `distribution_id`, `playerMinLevel` e outros).
- **Saída:** os parâmetros compilam como `<set name="..." val="..." />`, na ordem do documento.

**Blocked by:** 05 (Primeira zona de ponta a ponta)

**Status:** ready-for-agent

- [ ] O tipo só aceita os 23 valores, e a caixa é preservada no XML
- [ ] Um parâmetro conhecido mostra o valor padrão e só aceita valores do tipo dele
- [ ] Parâmetros livres compilam como `<set>` com `val`, na ordem em que foram adicionados
- [ ] Teste do seam Documento: `Siege` em vez de `SIEGE` não pode ser definido como tipo
