# 03: Pegada só no chão

**What to build:** em `ground()` (`internal/render/ground.go`), a tinta de dentro da faixa, as hachuras de acima e abaixo e as linhas de nível só valem em fragmentos de chão: `smoothstep(0.3, 0.5, abs(n.z))` vezes "olho acima do fragmento" (`uEye.z ≥ vPos.z`). Paredes e tetos ficam com a cor original. O contorno da zona sobre qualquer superfície (`edge`) continua como está (spec D3). `uEye`, que já existe no shader da cena, passa a ser usado pelo trecho `groundGLSL`.

**Blocked by:** None (can start immediately)

**Status:** needs-triage

- [ ] No app, no 23_18, com a câmera dentro da torre e a zona selecionada: as paredes e o teto internos não ficam tingidos, e o piso interno dentro da faixa fica
- [ ] Morro do 22_22: as hachuras quente e fria e as linhas de nível continuam no terreno, como antes
- [ ] O contorno da zona continua visível onde cruza uma parede

## Comments
