# 07: Diálogos nativos e integração final

**What to build:** títulos/descrições fornecidos ao Windows para escolher arquivos/pastas seguem idioma ativo no instante da abertura; restante controlado pelo Windows. Cobrir texto residual exposto fora dos tickets anteriores, atualizar documentação de uso (controle/preferência) e verificar fluxo integrado. Não traduzir nomes, caminhos, XML nem formato do projeto. Smoke da janela real: início pt-BR, projeto com problema/aviso, mapa carregando, trocar EN com altura/XML abertos, persistir/reabrir EN e voltar pt-BR; comparar XML copiado.

Blocked by: 02, 03, 04, 05, 06, 08

Status: claimed

- [x] Diálogos usam títulos/descrições no idioma atual; escolha de idioma não altera projeto.
- [x] Verificações integradas observam estado, contagem/ordem/alvo de problemas, aviso e estabilidade do XML.
- [ ] Smoke visual na janela padrão comprova legibilidade/interação e preferência após reinício.

## Comments

Após a revisão, `go test ./...`, `go vet ./...` e o build passaram. No Windows, o projeto de smoke carregou o tile 22_22 em 1280×800. Sem preferência, abriu em PT-BR; com preferência `en`, abriu em EN. Com controle interativo autorizado, selecionei a zona com aviso e a zona com problema, confirmei os destinos distintos, deixei apenas a zona válida para compilação e obtive XML apesar do aviso. Com janela de altura e XML abertas, inspetor rolado e campo de folga Z focado com `512`, alternei PT-BR → EN: títulos, causas, medidas, status de cópia e aviso mudaram, mas campo, foco, seleção, janelas e rolagem permaneceram; o XML copiado antes/depois foi idêntico (267 bytes). O diálogo nativo **Open project** exibiu título e filtro `Zone Builder project (*.zbproj)` em inglês, com o restante em português do Windows. A preferência `en` foi gravada na configuração isolada e reapareceu ao reabrir. A captura final após encurtar as legendas inglesas da régua mostra os dois rótulos inteiros dentro da janela de altura em 1280×800.

Pendente para fechar o smoke: alternar enquanto a carga de mapa ainda estiver em andamento e abrir também os diálogos de salvar e escolher pasta. O controle humano do desktop foi revogado ao mudar o foco e uma nova solicitação expirou; não substituir essa prova por inferência nem marcar o critério final como concluído.
