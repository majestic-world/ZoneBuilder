# 08: Ferramentas e mensagens do editor de zonas

**What to build:** localizar todos os textos exibidos produzidos pelo `zoneEditor` em `cmd/zonebuilder` (`shapetool.go`, `zonetool.go`, `edittool.go`, `suggest.go`, `ground.go`, `zonelist.go`, `zarrow.go`, `worstpins.go` e auxiliares correlatos): desenho, exclusões, seleção, ajuste ao chão, undo/redo, instrução da ferramenta e ações do viewport. Guardar mensagens como chave e dados até a apresentação no idioma corrente, sem alterar geometria nem o andamento de desenho/medição; dados de usuário e tokens do servidor literais. Evitar editar `main.go` e os arquivos do ticket 05; combine o contrato de apresentação com ele.

**Blocked by:** 01, 02

**Status:** resolved

- [x] Troca durante polígono ou medição preserva andamento e instrução muda imediatamente.
- [x] Mensagens anteriores ainda visíveis reformatam; ações não são repetidas.
- [x] Operações, undo/redo, seleção e XML dão os mesmos resultados nos dois idiomas.

## Comments

- 2026-10-09: Claimed in `i18n/08-editor` from `c918e4b`.
- 2026-10-09: Catálogo `editor` em pt-BR/en, estados de ferramenta e pinos derivados por idioma, `LastMessage` e `Status(lang)` guardam identidade e dados sem repetir ações; testes de troca e histórico incluídos. Verificação conjunta de testes/build/lint fica com a integração.
