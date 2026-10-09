# 13: Verificação da folga em jogo

**What to build:** conferir no servidor que a cobertura medida no app vale em jogo. No pior ponto que o app relatar (pino do topo), ficar em cima com o GM e comparar o `//pos` com o Z relatado. Depois, criar uma zona com folga do topo entre 0 e 32 sobre um morro, compilar, reiniciar o servidor e conferir se o servidor reconhece o personagem dentro dela no pico. O resultado confirma `MinClearance = 32` e os limiares dos avisos, ou reabre o ADR 0003.

**Blocked by:** 08, 11

**Status:** ready-for-human

- [ ] A diferença entre o `//pos` e o Z relatado no pior ponto está registrada nos comentários; acima de 16, o ADR 0003 e `MinClearance` são reabertos
- [ ] O teste da zona com folga apertada está registrado, com o valor da folga e se o servidor reconheceu o personagem
