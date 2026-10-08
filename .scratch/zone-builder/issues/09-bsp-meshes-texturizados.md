# 09: BSP e meshes texturizados

**What to build:** BSP e static meshes aparecem texturizados como no jogo.
- **Grafo de material:** Shader, FinalBlend e Combiner são seguidos até 8 níveis até a primeira Texture, com `Skins[i]` do ator valendo mais que `Materials[i]` da mesh.
- **Texturas P8:** decodificadas com a Palette, o que o UE2-Studio não faz.
- **Passes, na ordem do UE2-Studio:** Opaque, Masked (corte em alfa 0,5), TerrainLayer, Translucent, Brighten, Modulated, Additive, Water, Overlay.

**Blocked by:** 06 (BSP no viewport e no picking), 07 (Static meshes no viewport e no picking), 08 (Terreno texturizado)

**Status:** resolved

- [x] Comparação lado a lado com o modo Textured do UE2-Studio em 3 vistas (terreno aberto, interior BSP, área densa de meshes) sem diferença visível de material
- [x] Nenhum material cai no fallback sem textura onde o UE2-Studio desenha textura
- [x] Folhagem e grades usam transparência por máscara, e a água usa o pass Water
- [x] Teste do seam Cena: uma textura P8 real do cliente decodifica para os pixels esperados

## Comments

Resolvido na branch de integração `zone-builder`. Branch `zb/09-bsp-meshes-texturizados`. 3 vistas comparadas com render offscreen do UE2-Studio; censo de materiais em 309 mapas: 0 sem textura onde o UE2-Studio tem textura. Teste de P8 contra `assets/*.rgba` do UE2-Studio.
