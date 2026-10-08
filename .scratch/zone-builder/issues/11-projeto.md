# 11: Salvar e abrir projeto

**What to build:** o usuário salva o trabalho num arquivo de projeto e reabre depois, inclusive com zonas incompletas.
- **Arquivo de projeto (JSON):** caminho do cliente, pasta de saída do XML, tiles abertos, zonas, e cor e visibilidade por zona.
- **Configuração do usuário:** guarda à parte a pasta do cliente, a pasta de saída e os mapas recentes, para não configurar tudo a cada abertura.

**Blocked by:** 05 (Primeira zona de ponta a ponta)

**Status:** resolved

- [x] Salvar, fechar e reabrir o projeto devolve as mesmas zonas, os mesmos tiles abertos e a mesma pasta de saída
- [x] Uma zona incompleta (polígono com 2 vértices) é salva e reaberta sem perda
- [x] Ao abrir o app, a pasta do cliente e os mapas recentes já estão preenchidos
- [x] Teste do seam Documento: salvar e carregar devolve o documento idêntico, inclusive com zona incompleta

## Comments

Resolvido na branch de integração `zone-builder`. Branch `zb/11-projeto`. Projeto `.zbproj` (JSON) e configuração em `%AppData%/ZoneBuilder/config.json`. Teste de ida e volta inclui zona incompleta, cor e visibilidade.
