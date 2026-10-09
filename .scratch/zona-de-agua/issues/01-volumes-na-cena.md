# 01: Volumes de água na cena

**What to build:** ao abrir um tile, a cena passa a ter os volumes de água vivos dele (`Scene.WaterVolumes`), com faces, planos, caixa, topo, fundo e as marcas `Exact` e `Unsupported`, em coordenadas do cliente. A geometria sai dos nós BSP do Model apontado por `Brush`, todos eles, sem o filtro de visibilidade, e nunca de `Model.Points` (spec D1). A transformação é a do ABrush: `Location + PostScale · R(Rotation) · MainScale · (v − PrePivot)`. Para isso, `l2pkg` lê o struct `Scale` como lista aninhada, `unreal` ganha `ReadBrush`, `BrushTransform` e `Model.Polygons` (com `VisiblePolygons` reescrito sobre ele), e `scene` carrega os volumes depois do BSP (spec D2). Nada aparece na UI nesta fatia.

**Blocked by:** None (can start immediately)

**Status:** needs-triage

- [ ] Teste com o cliente real (pulado sem `ZB_CLIENT`): 22_22 tem 8 volumes vivos, e o `WaterVolume0` tem caixa x 86.757…98.304, y 152.256…153.600, z −8.778…−3.780
- [ ] Teste com o cliente real: o `MainScale` e o `PostScale` de um `WaterVolume` de 22_22 decodificam como `(1, 1, 1)` com `SheerRate 0`
- [ ] Teste do seam: `BrushTransform` com `PrePivot`, `MainScale`, rotação de 90° em Yaw e `PostScale` não uniforme leva um vértice conhecido ao ponto calculado à mão
- [ ] Teste do seam: um volume com `SheerRate ≠ 0` sai marcado `Unsupported`, com o motivo
- [ ] Teste do seam: hexaedro com 1 parede inclinada sai com `Exact = false`; caixa alinhada sai com `Exact = true`
- [ ] Os testes de materiais e BSP de hoje continuam passando com `VisiblePolygons` sobre `Model.Polygons`
- [ ] Abrir 22_22 no app não fica mais lento (medir o `Load` antes e depois no log)

## Comments
