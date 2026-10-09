# 03: Pegada só no chão

**What to build:** em `ground()` (`internal/render/ground.go`), a tinta de dentro da faixa, as hachuras de acima e abaixo e as linhas de nível só valem em fragmentos de chão: `smoothstep(0.3, 0.5, abs(n.z))` vezes "olho acima do fragmento" (`uEye.z ≥ vPos.z`). Paredes e tetos ficam com a cor original. O contorno da zona sobre qualquer superfície (`edge`) continua como está (spec D3). `uEye`, que já existe no shader da cena, passa a ser usado pelo trecho `groundGLSL`.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] No app, no 23_18, com a câmera dentro da torre e a zona selecionada: as paredes e o teto internos não ficam tingidos, e o piso interno dentro da faixa fica
- [x] Morro do 22_22: as hachuras quente e fria e as linhas de nível continuam no terreno, como antes
- [x] O contorno da zona continua visível onde cruza uma parede

## Comments

Implementado na branch `zo/03-pegada-so-no-chao` e integrado em `zona-oca-e-faixa-pelo-clique`. Em `ground()`, a tinta, as hachuras e as linhas de nível agora são multiplicadas por `up * step(vPos.z, uEye.z)`. O `edge` não mudou.

Capturas antes e depois:
- Dentro da torre do 23_18: as paredes e o teto ficam com a cor original, e o piso continua tingido.
- Morro do 22_22: as imagens são idênticas.
- O contorno continua subindo pela parede.

Evidências: `C:/Workspace/zone-builder-notes/zona-oca-e-faixa-pelo-clique/03-evidence/`.
