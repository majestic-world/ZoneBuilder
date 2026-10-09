# 04: Menu "Compilar zona de água"

**What to build:** um clique direito curto no viewport (Press com só o botão secundário e Release dentro de `clickSlop`) sobre a água abre um menu de contexto na posição do ponteiro com o item **"Compilar zona de água"** (ícone Lucide `droplets`, que ainda não está em `internal/ui/icon/lucide/` e entra junto, com a mesma licença ISC). Se a água clicada está fora da seleção, ela é selecionada antes, com a mesma regra do ticket 02; fora da água, nada abre. Arrastar com o botão direito continua girando a câmera, e esquerdo + direito continua sendo o lift. O menu é `ui.ContextMenu`, extraído do `projectMenu`, que passa a usá-lo; ele fecha com Esc, com clique fora e ao escolher o item, e é desenhado por último no `Shell.Layout` (spec D4). A ação aplica os comandos dos planos do ticket 03 num `zone.Batch` (1 passo de desfazer), compila só essas zonas com `Document.Compile(ids)`, sem mexer na seleção de compilação da lista, e abre a janela de XML. O status diz quantas zonas foram criadas ou reaproveitadas e lista os avisos. Com o polígono de outra zona aberto, o item fica desabilitado e o status explica (spec D5).

**Blocked by:** 02 (Clique seleciona a água), 03 (Volume vira zona de água)

**Status:** ready-for-agent

- [ ] Teste do seam: aplicar os planos de 3 volumes e dar 1 `Undo` deixa o documento como antes
- [ ] No app, 22_22: clique direito no mar de Giran → "Compilar zona de água" → a janela de XML mostra `zonebuilder_water.xml` com 1 zona `[22_22_WaterVolume0]` e 8 polígonos, e o Copiar copia esse texto
- [ ] No app, a zona aparece na lista de zonas, sobrevive a salvar e reabrir o projeto e some com Ctrl+Z
- [ ] No app, compilar a mesma água de novo não cria zona nova, e o status diz que compilou a versão do projeto
- [ ] No app, 22_24: fonte alta + Ctrl+clique na baixa → 2 zonas no XML
- [ ] No app, 25_25: compilar o `WaterVolume9` mostra o aviso de água sobreposta com o `WaterVolume7`
- [ ] No app, arrastar com o botão direito gira a câmera e não abre menu; clique direito no terreno seco não abre menu
- [ ] O menu do projeto continua abrindo, fechando e executando os itens como antes

## Comments
