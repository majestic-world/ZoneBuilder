# 10: Vários tiles e desempenho

**What to build:** o usuário abre um tile junto com os vizinhos (até 3×3) e demarca uma zona que cruza a borda entre tiles sem trocar de mapa.
- **Carregamento:** em segundo plano, com progresso visível; a janela continua respondendo e o upload para a GPU acontece na thread do contexto.
- **Memória:** tiles distantes são descarregados.
- **Culling:** frustum culling por batch, com os batches divididos por setor do tile.

**Blocked by:** 09 (BSP e meshes texturizados)

**Status:** resolved

- [x] Abrir 3×3 tiles em torno de Giran mostra o progresso do carregamento sem travar a janela
- [x] Com 3×3 tiles abertos, o viewport mantém pelo menos 60 fps com a câmera parada e voando
- [x] Um polígono com vértices em 2 tiles vizinhos é demarcado e compilado sem trocar de mapa
- [x] Voar para longe descarrega os tiles distantes, e a memória do processo para de crescer

## Comments

Resolvido na branch de integração `zone-builder`. Branch `zb/10-varios-tiles`. 3×3 em Giran: 110–127 fps parado e ≥ 89 fps voando (sem vsync, RTX 4070 Ti SUPER); memória volta a cerca de 2,7 GB ao descarregar. Polígono na borda 22_22/23_22 compilado e carregado no harness. Manual: `//zone_check` no jogo.
