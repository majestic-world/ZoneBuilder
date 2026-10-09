# 01: Volumes de água na cena

**What to build:** ao abrir um tile, a cena passa a ter os volumes de água vivos dele (`Scene.WaterVolumes`), com faces, planos, caixa, topo, fundo e as marcas `Exact` e `Unsupported`, em coordenadas do cliente. A geometria sai dos nós BSP do Model apontado por `Brush`, todos eles, sem o filtro de visibilidade, e nunca de `Model.Points` (spec D1). A transformação é a do ABrush: `Location + PostScale · R(Rotation) · MainScale · (v − PrePivot)`. Para isso, `l2pkg` lê o struct `Scale` como lista aninhada, `unreal` ganha `ReadBrush`, `BrushTransform` e `Model.Polygons` (com `VisiblePolygons` reescrito sobre ele), e `scene` carrega os volumes depois do BSP (spec D2). Nada aparece na UI nesta fatia.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] Teste com o cliente real (pulado sem `ZB_CLIENT`): 22_22 tem 8 volumes vivos, e o `WaterVolume0` tem caixa x 86.757…98.304, y 152.256…153.600, z −8.778…−3.780
- [x] Teste com o cliente real: o `MainScale` e o `PostScale` de um `WaterVolume` de 22_22 decodificam como `(1, 1, 1)` com `SheerRate 0`
- [x] Teste do seam: `BrushTransform` com `PrePivot`, `MainScale`, rotação de 90° em Yaw e `PostScale` não uniforme leva um vértice conhecido ao ponto calculado à mão
- [x] Teste do seam: um volume com `SheerRate ≠ 0` sai marcado `Unsupported`, com o motivo
- [x] Teste do seam: hexaedro com 1 parede inclinada sai com `Exact = false`; caixa alinhada sai com `Exact = true`
- [x] Os testes de materiais e BSP de hoje continuam passando com `VisiblePolygons` sobre `Model.Polygons`
- [x] Abrir 22_22 no app não fica mais lento (medir o `Load` antes e depois no log)

## Comments

Implementado na branch `za/01-volumes-na-cena` e integrado em `zona-de-agua`. Testes: `TestGiranWaterVolumesComeFromTheBrushFaces`, `TestWaterVolumeScaleDecodesAsTaggedStruct` (vermelho antes da mudança em `l2pkg`), `TestBrushTransformAppliesTheABrushOrder`, `TestWaterVolumeWithShearIsUnsupported`, `TestWaterVolumeExactOnlyForVerticalWallsAndFlatCaps`. `Load` de 22_22: 292/276/277 ms antes, 268/271/276 ms depois. Varredura de todos os mapas: 674 volumes vivos em 202 tiles (os 673 da sonda mais `17_13_classic`), 0 `Unsupported`, 13 não exatos.

Evidências: `C:/Workspace/zone-builder-notes/zona-de-agua/01-evidence/`.
