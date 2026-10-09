# 02: Chão em 3 estados no viewport

**What to build:** a pegada da zona selecionada no chão fica sempre ligada; o botão Chão passa a controlar só a grade das células. O chão dentro do contorno e da faixa continua com a tinta da zona, o chão acima do topo ganha uma hachura quente e o chão abaixo do piso uma hachura fria, em outra direção, com cores fixas diferentes de qualquer cor de zona. Exclusões continuam como buracos. Sobre o chão dentro do contorno, uma linha marca onde ele cruza `zmax` e outra onde cruza `zmin`, que é a fronteira exata da área descoberta (spec D4a a D4c). Quando a zona passa do limite de shapes ou pontos que o shader comporta, o inspetor avisa "pegada parcial" com quantos shapes ficaram fora do desenho, em vez de omitir em silêncio.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] Captura de tela do app com a zona da captura: o morro que fura o topo aparece com a hachura quente e com a linha do topo contornando o morro
- [ ] Captura de tela com a faixa erguida acima do chão: o chão aparece com a hachura fria, diferente da quente
- [ ] Com o botão Chão desligado, a pegada continua visível e só a grade some
- [ ] Uma zona com mais shapes do que o shader comporta mostra o aviso de pegada parcial no inspetor
