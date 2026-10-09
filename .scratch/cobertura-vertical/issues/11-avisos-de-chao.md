# 11: Avisos de chão no painel de problemas

**What to build:** o painel de problemas ganha a categoria aviso, com ícone próprio, que não bloqueia a compilação (decisão adotada; spec D6). Os avisos são: chão acima do topo (1% ou mais da área de chão, ou folga do topo abaixo de −16), chão abaixo do piso (a mesma regra), folga apertada (folga entre 0 e 32) e área sem chão medido. Chão sob exclusão não gera aviso. Os limiares são constantes nomeadas. Clicar num aviso seleciona a zona e o shape e leva a câmera ao pior ponto. A contagem de problemas da lista de zonas continua contando só os erros.

**Blocked by:** 01, 05

**Status:** resolved

- [x] Uma zona com o morro furando o topo compila e mostra o aviso "chão acima do topo"
- [x] Clicar no aviso enquadra o pico
- [x] Uma lasca de 1 célula num penhasco, abaixo dos limiares, não gera aviso
- [x] Um shape com uma exclusão sobre um morro não gera aviso pelo chão sob a exclusão

## Comments

Implementado na branch `cv/11-avisos-de-chao` e integrado em `cobertura-vertical`. Seam: morro furando o topo avisa no pico; lasca de 1 célula abaixo dos limiares não avisa; exclusão sobre o morro não avisa. Smoke: a zona compila com o aviso "chão acima do topo", o clique enquadra o pico, e a lista de zonas continua contando só erros.

Evidências: `C:/Workspace/zone-builder-notes/cobertura-vertical/11-evidence/`.
