# 11: Física do modo jogo

**What to build:** o módulo puro `play` porta a física e a câmera do Play Map do UE2-Studio com as mesmas constantes (spec D8): BVH de triângulos (divisão pela mediana, até 12 por folha), cápsula (raio 16, altura 80, olho a 64), gravidade 980, pulo de 360, corrida de 260 (×2 com Shift), voo de 900 sem colisão, sub-passos de até 4 unidades, resolução da penetração em até 6 iterações, chão quando `n.z > 0,65`, escorregar em rampa íngreme, e câmera em terceira pessoa com braço de 240 e varredura de esfera de raio 6. A entrada é uma sequência de comandos (andar, correr, pular, voar, olhar) por passo de tempo; a saída é a posição, a velocidade, o estado da cápsula (no chão, caindo, pulando, voando) e a câmera.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] Teste do seam: a cápsula solta acima de um plano para em pé sobre ele; andando contra uma parede, para nela
- [x] Teste do seam: uma rampa de 30° é subida e uma de 60° não
- [x] Teste do seam: o pulo sobe e volta ao chão, sem lançamento extra em contato com degrau
- [x] Teste do seam: o voo atravessa a parede
- [x] Teste do seam: o braço da câmera encurta atrás de uma parede

## Comments

Integrado em `popular-zona`, a partir de `pz/11-fisica-do-modo-jogo` (`6c6498c`). Testes de aterrissagem/parede, rampas de 30°/60°, pulo e degrau sem lançamento extra, voo sem colisão e braço da câmera passaram. Build e vet passaram com `ZB_CLIENT`. Em faces íngremes, a resolução usa a componente horizontal para impedir subida da rampa de 60°; diferença do port literal registrada nas notas e no plano.

API e evidências: `C:/Workspace/zone-builder-notes/popular-zona/11-fisica-do-modo-jogo.md`.
