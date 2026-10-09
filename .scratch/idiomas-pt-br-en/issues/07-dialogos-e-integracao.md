# 07: Diálogos nativos e integração final

**What to build:** títulos/descrições fornecidos ao Windows para escolher arquivos/pastas seguem idioma ativo no instante da abertura; restante controlado pelo Windows. Cobrir texto residual exposto fora dos tickets anteriores, atualizar documentação de uso (controle/preferência) e verificar fluxo integrado. Não traduzir nomes, caminhos, XML nem formato do projeto. Smoke da janela real: início pt-BR, projeto com problema/aviso, mapa carregando, trocar EN com altura/XML abertos, persistir/reabrir EN e voltar pt-BR; comparar XML copiado.

Blocked by: 02, 03, 04, 05, 06, 08

Status: claimed

- [x] Diálogos usam títulos/descrições no idioma atual; escolha de idioma não altera projeto.
- [x] Verificações integradas observam estado, contagem/ordem/alvo de problemas, aviso e estabilidade do XML.
- [ ] Smoke visual na janela padrão comprova legibilidade/interação e preferência após reinício.

## Comments

No ramo de integração, `go test ./...`, `go vet ./...` e `go build -o "bin/Zone Builder i18n reviewed.exe" ./cmd/zonebuilder` passaram após a revisão. No Windows, o binário isolado abriu o projeto de smoke e carregou o tile 22_22 em 1280×800; capturas mostraram PT-BR por padrão e EN ativo ao iniciar com preferência inglesa, com os controles superiores legíveis e sem sobrepor viewport ou inspetor. A troca interativa, os diálogos, janelas flutuantes, avisos e a comparação do XML copiado ainda dependem do controle da janela: a entrada de mouse em segundo plano não alterou a UI Gio e a solicitação de controle humano expirou sem aprovação. Não marcar o smoke como concluído até exercitar essas ações.
