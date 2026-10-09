# 05: Ações globais, carga e status transitório

**What to build:** migrar mensagens exibidas pelo loop e pelos auxiliares globais de `cmd/zonebuilder` (`main.go`, `project.go`, `tiles.go`, `water.go`, `cursor.go`: abrir/salvar/copiar/compilar, seleção de água, carga/progresso, erros recuperáveis e status do cursor). Guardar identidade de mensagem com parâmetros/detalhe técnico para reformatar itens visíveis depois da troca; não repetir comando, carga, validação ou medição. Em falhas, acrescentar contexto traduzido e conservar detalhe original. Mensagens internas do editor de zonas pertencem ao ticket 08; problemas ao 03.

**Blocked by:** 01, 02

**Status:** ready-for-agent

- [ ] Status produzido antes da troca muda de idioma; erro técnico mantém detalhe útil.
- [ ] Alternar durante carga, desenho ou medição não reinicia trabalho nem modifica geometria.
- [ ] Preservar estado e XML byte a byte; não traduzir logs ou protocolo.

## Comments
