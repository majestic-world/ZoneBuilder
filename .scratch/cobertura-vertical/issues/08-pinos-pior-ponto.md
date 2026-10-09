# 08: Pinos do pior ponto

**What to build:** o shape selecionado ganha um pino no chão mais alto e outro no chão mais baixo sob o contorno, cada um com um rótulo da folga (`topo +412`, `piso −96`), em vermelho quando a folga é negativa. Clicar no rótulo leva a câmera ao ponto, como o clique num problema (spec D4e). Os pinos se movem junto com a faixa durante o arrasto da seta Z.

**Blocked by:** 01

**Status:** resolved

- [x] Captura de tela da zona da captura: o pino do topo está no pico que fura o topo, com o rótulo em vermelho
- [x] Clicar no rótulo enquadra o pico
- [x] Arrastar a seta Z muda o número do rótulo a cada frame

## Comments

Implementado na branch `cv/08-pinos-pior-ponto` e integrado em `cobertura-vertical`. Smoke: rótulo "topo −815" em vermelho no pico da captura; clicar enquadra o pico; arrastar a seta Z muda o rótulo a cada frame. Os pinos somem quando a zona selecionada está oculta (revisão).

Evidências: `C:/Workspace/zone-builder-notes/cobertura-vertical/08-evidence/`.
