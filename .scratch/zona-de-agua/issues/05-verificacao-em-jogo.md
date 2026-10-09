# 05: Verificação em jogo do offset da água

**What to build:** confirmar no servidor que a zona compilada pelo app nada, respira e pesca igual à água do cliente. Pôr no datapack a zona de um lago que não tem zona de água no `water.xml` (ou trocar a de 22_22 pela do app, removendo a antiga), reiniciar o servidor e, com o GM:

1. descer andando da margem para dentro da água e anotar o `//pos` no momento em que o personagem passa a nadar no cliente e no momento em que o servidor aplica a velocidade de nado;
2. mergulhar até aparecer a barra de fôlego e anotar o `//pos`;
3. pescar na beira (com uma zona `FISHING` sobre ela) e ver se a boia fica na superfície.

Se a diferença entre o nado no cliente e o nado no servidor passar de 16 unidades, reabrir o ADR 0005 com a medida e trocar `water.ServerZOffset`.

**Blocked by:** 04 (Menu "Compilar zona de água")

**Status:** ready-for-human

- [ ] Medidas dos 3 passos anotadas aqui, com o `//pos` e o topo do volume
- [ ] ADR 0005 confirmado ou corrigido com a medida

## Comments
