# 09: Compilação do XML de spawn

**What to build:** o módulo `spawnxml`, no molde de `zonexml`, e `spawn.Document.Compile(fileName) (File, error)` com `*BlockedError` como em `zone` (spec D2 e D4): cabeçalho `<?xml version="1.0" encoding="utf-8"?>`, `<!DOCTYPE list SYSTEM "spawn.dtd">` e `<list>`, indentação de tab, fim de linha `\n`; um `<spawn name="[<nome>_<i>]">` por ponto (i a partir de 0) com `<npc id count="1" respawn [respawn_rand] pos="x y z h" />`, `respawn_rand` só quando > 0, heading nunca 0, áreas em ordem de ID e pontos na ordem do documento, escape XML no nome, e nunca `<mesh>`, `event_name`, `period_of_day`, `respawn_cron` nem `ai_params`. No modo população, "Compilar XML" compila todas as áreas num arquivo de nome livre (padrão: o nome do projeto) e mostra o resultado na janela de XML existente, com o botão de copiar; problemas que bloqueiam impedem a compilação como no modo de zonas.

**Blocked by:** 08

**Status:** ready-for-agent

- [ ] Teste do seam: o XML tem 1 `<spawn>` por ponto, `count="1"`, heading nunca 0, `respawn_rand` só quando > 0 e nenhum `<mesh>`; compilar 2 vezes dá bytes idênticos
- [ ] Teste do seam: área desatualizada ou com problema que bloqueia faz `Compile` devolver `*BlockedError`; avisos de distribuição não bloqueiam
- [ ] Smoke no app: Compilar XML no modo população abre a janela de XML com o arquivo, e o botão de copiar põe o texto exato no clipboard
- [ ] O XML de um projeto com 3 áreas é carregado pelo `SpawnParser` num harness no molde de `zone-builder-notes/zoneparser-harness`, sem exceção
