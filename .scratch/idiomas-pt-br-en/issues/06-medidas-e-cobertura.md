# 06: Medidas, chão e cobertura

**What to build:** localizar a apresentação de perfis, aviso de cobertura, altura, chão/outras camadas, folgas do piso e topo, cobertura, áreas e pior ponto no inspetor e janela; formatar números apresentados (milhares, decimais, percentual, negativo, plural) pelo idioma. Usar terminologia do glossário; chão é superfície e piso é limite zmin. Não alterar cálculo, offset de água, geometria, regra de aviso ou entrada numérica. Mensagens existentes na tela mudam sem recalcular.

Blocked by: 01, 03, 04

Status: resolved

- [x] Valores que distinguem separadores e plural em pt-BR/en aparecem corretamente, inclusive área sem chão e folga negativa.
- [x] Avisos continuam não bloqueantes e mudam de idioma com a lista aberta.
- [x] Dados de medição e XML permanecem idênticos ao alternar.

## Comments

- Apresentação de cobertura e régua recebe `locale.Language` em cada frame, reaproveita o relatório/perfil armazenado e traduz títulos, linhas, porcentagens, áreas, camadas, folgas e marcas; os números do eixo usam o mesmo idioma. Pinos/rótulos do viewport são responsabilidade do ticket 08; avisos não bloqueantes e lista reativa vieram do ticket 03.
- Os testes de apresentação exercitam pt-BR → en no mesmo relatório com milhares, decimais, plural, área sem chão e folga negativa; o teste de avisos/compilação do ticket 03 cobre que avisos não entram nos problemas bloqueantes. Nenhuma entrada, dado de medição ou caminho de XML foi alterado. Verificações ficam para a integração.
