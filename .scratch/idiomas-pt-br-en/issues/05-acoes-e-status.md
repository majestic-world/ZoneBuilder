# 05: Ações globais, carga e status transitório

**What to build:** migrar mensagens exibidas pelo loop e pelos auxiliares globais de `cmd/zonebuilder` (`main.go`, `project.go`, `tiles.go`, `water.go`, `cursor.go`: abrir/salvar/copiar/compilar, seleção de água, carga/progresso, erros recuperáveis e status do cursor). Guardar identidade de mensagem com parâmetros/detalhe técnico para reformatar itens visíveis depois da troca; não repetir comando, carga, validação ou medição. Em falhas, acrescentar contexto traduzido e conservar detalhe original. Mensagens internas do editor de zonas pertencem ao ticket 08; problemas ao 03.

Blocked by: 01, 02

Status: resolved

- [x] Status produzido antes da troca muda de idioma; erro técnico mantém detalhe útil.
- [x] Alternar durante carga, desenho ou medição não reinicia trabalho nem modifica geometria.
- [x] Preservar estado e XML byte a byte; não traduzir logs ou protocolo.

## Comments

- 2026-10-09: Ticket assumido em `i18n/05-actions`, base `c918e4b`.
- 2026-10-09: Ações/projeto/água/carga/cursor migrados ao catálogo `actions`; status guarda `locale.Message` com dados estruturados quando precisa recomposição. A troca reapresenta estado em memória, sem repetir operação, perfil ou carga. Erros mantêm detalhe técnico com contexto traduzido; logs e XML não mudam. Testes adicionados para alternância de resultados, carga, água e invariância do documento/XML; execução fica para a integração.
- 2026-10-09: Integração consumirá `zoneEditor.Status(lang)` e mensagens editoriais (08), `cover.inspector(..., lang)`/`cover.heightWindow(..., lang)` (06), `pinMessage`/`pinStatus`/`pinLabels(..., lang)` (08), além dos contratos de problemas (03) e painéis (04) já incorporados.
- 2026-10-09: Chaves em `action`/`actionArgs`/`actionError`/`waterAction` agora são `locale.Message` literais nos callsites, inclusive o plural de compilação bloqueada, para que `locale.Check` descubra todas as referências e detecte tradução ausente.
