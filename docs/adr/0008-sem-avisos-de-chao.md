# Sem avisos de chão

Substitui a **Decisão 2** do [ADR 0004](0004-chao-medido-e-avisos-sem-bloqueio.md), que listava no painel de problemas os avisos de chão (chão acima do topo, chão abaixo do piso, folga apertada e área sem chão medido), sem bloquear a compilação.

Na prática os avisos viraram ruído no painel: uma zona de água, por exemplo, listava 2 avisos por shape, e o usuário não precisava deles. A mesma informação já aparece onde a zona é editada: as hachuras e a tinta no chão, os pinos no pior ponto com a folga, os números do inspetor e a régua da janela de altura.

## Decisão

Os avisos de chão foram removidos, em todas as zonas. O painel de problemas lista só os problemas que bloqueiam a compilação (`Document.Problems()`); chão fora da faixa continua sem bloquear.

Continua valendo todo o resto do ADR 0004 e do [ADR 0006](0006-terreno-como-camada.md): a medição sobre os triângulos, a classificação pela faixa, os extremos e as folgas, as outras camadas, as hachuras, os pinos, a régua e os ajustes ao chão. Os avisos das áreas de spawn, de carga da cena e da compilação da água não mudam.

## Consequences

- Os limiares `StrayShare`, `StrayDepth` e `MinClearance` saíram do código. Uma folga entre 0 e 32 não é mais destacada; quem quiser margem para o erro do ADR 0003 olha a folga nos pinos e na régua.
- Chão fora da faixa só é visto com a zona selecionada: zonas não selecionadas não são mais medidas para alimentar o painel.
