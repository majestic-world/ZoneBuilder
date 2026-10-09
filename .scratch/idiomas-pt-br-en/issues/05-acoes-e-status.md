# 05: Mensagens de ações e estado transitório

**What to build:** migrar mensagens exibidas pelo loop e auxiliares de `cmd/zonebuilder` (abrir/salvar/copiar/compilar, seleção/edição/água, carga/progresso, instrução da ferramenta, erros recuperáveis e status do cursor). Guardar identidade de mensagem com parâmetros/detalhe técnico para reformatar itens visíveis depois da troca; não repetir comando, carga, validação ou medição. Em falhas, acrescentar contexto traduzido e conservar detalhe original. Coordenar com ticket 02 no loop de janela e 03 nas linhas.

**Blocked by:** 01, 02, 03

**Status:** ready-for-agent

- [ ] Status produzido antes da troca muda de idioma; erro técnico mantém detalhe útil.
- [ ] Alternar durante carga, desenho ou medição não reinicia trabalho nem modifica geometria.
- [ ] Preservar estado e XML byte a byte; não traduzir logs ou protocolo.

## Comments
