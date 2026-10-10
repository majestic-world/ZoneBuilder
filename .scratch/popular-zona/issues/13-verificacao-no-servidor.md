# 13: Verificação no servidor e em jogo

**What to build:** a prova de que o XML compilado vale no servidor (spec, Testing Decisions, "Servidor"): um projeto com 3 áreas geradas na clareira de referência é compilado; um harness fora do repositório, no molde de `zone-builder-notes/zoneparser-harness`, carrega o arquivo pelo `SpawnParser` sem exceção; e, depois de reiniciar o servidor com o arquivo em `data/spawn/`, o `//pos` ao lado de 3 monstros fica a menos de 16 unidades dos pontos da prévia, com o heading batendo (confirma ou corrige o **[INFERENCE]** heading = yaw da spec D7). Se o heading não bater, a conversão única entra no compilador e na prévia.

**Blocked by:** 09, 10

**Status:** ready-for-agent

- [ ] O harness carrega o XML de 3 áreas pelo `SpawnParser` sem exceção (log nas notas)
- [ ] Em jogo: 3 monstros a menos de 16 unidades dos pontos da prévia, com o heading conferido (manual, pelo usuário, se o agente não puder rodar o cliente)
