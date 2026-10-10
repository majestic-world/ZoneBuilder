# 13: Verificação no servidor e em jogo

**What to build:** a prova de que o XML compilado vale no servidor (spec, Testing Decisions, "Servidor"): um projeto com 3 áreas geradas na clareira de referência é compilado; um harness fora do repositório, no molde de `zone-builder-notes/zoneparser-harness`, carrega o arquivo pelo `SpawnParser` sem exceção; e, depois de reiniciar o servidor com o arquivo em `data/spawn/`, o `//pos` ao lado de 3 monstros fica a menos de 16 unidades dos pontos da prévia, com o heading batendo (confirma ou corrige o **[INFERENCE]** heading = yaw da spec D7). Se o heading não bater, a conversão única entra no compilador e na prévia.

**Blocked by:** 09, 10

**Status:** ready-for-human

- [x] O harness carrega o XML de 3 áreas pelo `SpawnParser` sem exceção (log nas notas)
- [ ] Em jogo: 3 monstros a menos de 16 unidades dos pontos da prévia, com o heading conferido (manual, pelo usuário, se o agente não puder rodar o cliente)

## Comments

Parte automatizável concluída em `popular-zona`: projeto com 3 áreas no 22_22, geração/compilação/cópia no app e harness Java com `LOADED 10 spawn(s)`, exit 0, sem exceção. A leitura do servidor sustenta a unidade e os eixos do heading; não substitui a verificação no cliente.

Não resolvido: falta reiniciar um servidor de teste com o XML e conferir posição e heading de 3 monstros no cliente conectado. Nenhum reinício ou alteração do servidor foi executado pelo agente. Roteiro, coordenadas e arquivo: `C:/Workspace/zone-builder-notes/popular-zona/13-verificacao-no-servidor.md` e `13-evidence/tres-areas.xml`. Após a conferência, registrar os resultados aqui e no ADR 0007 antes de resolver.
