# 07: Diálogos nativos e integração final

**What to build:** títulos/descrições fornecidos ao Windows para escolher arquivos/pastas seguem idioma ativo no instante da abertura; restante controlado pelo Windows. Cobrir texto residual exposto fora dos tickets anteriores, atualizar documentação de uso (controle/preferência) e verificar fluxo integrado. Não traduzir nomes, caminhos, XML nem formato do projeto. Smoke da janela real: início pt-BR, projeto com problema/aviso, mapa carregando, trocar EN com altura/XML abertos, persistir/reabrir EN e voltar pt-BR; comparar XML copiado.

Blocked by: 02, 03, 04, 05, 06, 08

Status: resolved

- [x] Diálogos usam títulos/descrições no idioma atual; escolha de idioma não altera projeto.
- [x] Verificações integradas observam estado, contagem/ordem/alvo de problemas, aviso e estabilidade do XML.
- [x] Smoke visual na janela padrão comprova legibilidade/interação e preferência após reinício.

## Comments

Após a revisão, `go test ./...`, `go vet ./...` e o build passaram. No Windows, o projeto de smoke carregou o tile 22_22 em 1280×800. Sem preferência, abriu em PT-BR; com preferência `en`, abriu em EN. Com controle interativo autorizado, selecionei a zona com aviso e a zona com problema, confirmei os destinos distintos, deixei apenas a zona válida para compilação e obtive XML apesar do aviso. Com janela de altura e XML abertas, inspetor rolado e campo de folga Z focado com `512`, alternei PT-BR → EN: títulos, causas, medidas, status de cópia e aviso mudaram, mas campo, foco, seleção, janelas e rolagem permaneceram; o XML copiado antes/depois foi idêntico (267 bytes). O diálogo nativo **Open project** exibiu título e filtro `Zone Builder project (*.zbproj)` em inglês, com o restante em português do Windows. A preferência `en` foi gravada na configuração isolada e reapareceu ao reabrir. A captura final após encurtar as legendas inglesas da régua mostra os dois rótulos inteiros dentro da janela de altura em 1280×800.

Smoke final no Windows com a janela de 1280×800 e configuração isolada: o diálogo de escolha de pasta apresentou a descrição fornecida pelo app `Lineage II client folder (containing Maps)` em inglês; **Save project** mostrou título e filtro `Zone Builder project (*.zbproj)` em inglês. Os controles restantes dos diálogos ficaram em português do Windows. Ambos foram cancelados, sem alterar o projeto por esses diálogos. Com vizinhos 3×3 habilitados, a carga de 25_25 mostrou `Loading: 0 of 9 tiles ready` e `Loading 25_25 and its neighbours...` depois da alternância PT-BR → EN; em seguida, os nove tiles e o viewport apareceram normalmente, com EN ainda selecionado e as 2 zonas preservadas. A alternância EN → PT-BR durante a carga anterior de 22_22 com vizinhos também preservou as zonas. O controle do desktop foi liberado e o executável temporário removido após o smoke.
