# 11: Avisos de chão no painel de problemas

**What to build:** o painel de problemas ganha a categoria aviso, com ícone próprio, que não bloqueia a compilação (decisão adotada; spec D6). Os avisos são: chão acima do topo (1% ou mais da área de chão, ou folga do topo abaixo de −16), chão abaixo do piso (a mesma regra), folga apertada (folga entre 0 e 32) e área sem chão medido. Chão sob exclusão não gera aviso. Os limiares são constantes nomeadas. Clicar num aviso seleciona a zona e o shape e leva a câmera ao pior ponto. A contagem de problemas da lista de zonas continua contando só os erros.

**Blocked by:** 01, 05

**Status:** ready-for-agent

- [ ] Uma zona com o morro furando o topo compila e mostra o aviso "chão acima do topo"
- [ ] Clicar no aviso enquadra o pico
- [ ] Uma lasca de 1 célula num penhasco, abaixo dos limiares, não gera aviso
- [ ] Um shape com uma exclusão sobre um morro não gera aviso pelo chão sob a exclusão
