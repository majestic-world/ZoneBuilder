# 07: Editor de áreas de spawn

**What to build:** no modo população, o usuário desenha áreas de spawn com as ferramentas de polígono, retângulo e círculo que já conhece (retângulo e círculo viram polígono), edita os vértices (arrastar, inserir no meio de uma aresta, apagar, mover a área inteira) e desfaz/refaz tudo pelo histórico do `spawn.Document`. A faixa Z da área nasce do chão sob o contorno, como nas zonas, e é ajustada na janela de altura flutuante que já existe. O painel do modo tem a lista de áreas (selecionar leva a câmera até a área, mostrar/ocultar, duplicar, apagar com desfazer) e as propriedades da área selecionada: nome, id do NPC, quantidade, respawn (60), `respawn_rand` (0), raio (padrão: o raio medido do monstro de prévia) e afastamento (32). Os problemas das áreas aparecem no painel de problemas, e um clique leva até a área. O `session` grava e restaura as zonas e as áreas no mesmo `.zbproj`, e o indicador de alterações não salvas considera os 2 documentos (spec D5). Textos novos no catálogo `spawn`, nos 2 idiomas, com contagens flexionadas.

**Blocked by:** 01, 02, 05

**Status:** ready-for-agent

- [ ] Smoke no app: 2 áreas criadas só com mouse e teclado (uma por polígono, outra por retângulo ou círculo), com vértices editados e desfazer/refazer funcionando
- [ ] Smoke no app: a faixa Z sugerida pelo chão aparece na criação e muda pela janela de altura
- [ ] Smoke no app: lista com selecionar (a câmera vai até a área), ocultar, duplicar e apagar com desfazer
- [ ] Smoke no app: uma área com id 0 aparece no painel de problemas, e o clique leva até ela
- [ ] Smoke no app: salvar, fechar e reabrir devolve as 2 áreas idênticas, e o indicador de não salvo reage a mudanças nas áreas
- [ ] O modo de zonas continua igual, e o desfazer de um modo não mexe no outro
- [ ] Interface nova traduzida em pt-BR e en
