# 03: Problemas de zona e avisos estruturados

**What to build:** separar dados de apresentação dos problemas de `internal/zone`: regras/códigos e parâmetros independentes do idioma; apresentar causas, valores e plural localizados em pt-BR/en. Migrar linhas de problemas da UI e `cmd/zonebuilder/problems.go`: recompor ao trocar apenas idioma, preservando ordem, quantidade, índices/alvos de clique e distinção entre erros bloqueantes e avisos de cobertura não bloqueantes. Não traduzir `Error()` por comparação de frases.

Blocked by: 01

Status: resolved

- [x] Validar/compilar produz o mesmo resultado e XML nos dois idiomas.
- [x] Linha já visível troca de idioma sem revalidar; clique continua selecionando a mesma zona/shape/vértice/restart.
- [x] Aviso de chão fica separado e não bloqueia compilação; contagem e mensagens em ambos os idiomas.

## Comments

- Claimed by `i18n/03-problems` at integration base `c918e4b`.
- Structured rules preserve targets and raw values; presentation lives in paired `zone` catalogs.
- `problemRows(ws, wsChanged, lang)` and `warningText(w, lang)` rebuild presentation only; the loop supplies the current language and calls `reformatProblemClick(lang)` for retained click status. Tests were added at document and presentation seams; integration runs checks after merge.
