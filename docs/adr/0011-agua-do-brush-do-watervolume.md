# Zona de água: geometria do brush BSP do `WaterVolume`, seleção por volume e compilação

O app gera a zona `water` a partir dos `WaterVolume` do cliente (`internal/water`). Este ADR registra de onde sai a geometria, como a água é escolhida no viewport e como os volumes viram zona. O deslocamento em Z (−30) está no [ADR 0005](0005-agua-30-abaixo-do-volume.md).

## O que o servidor faz com a zona

- O `zmax` da zona é a superfície da água: decide quando o personagem nada, quando perde fôlego e onde a boia de pesca fica (`Zones.java:132-139`, `ValidatePosition.java:104-110`, `FishingSkill.java:68-72`).
- A superfície é o maior `zmax` entre os shapes da zona (`Territory.java:20-33`, `Zones.java:132-139`). O servidor testa o Z por shape (`AbstractShape.java:7-9`).
- `Polygon.isInside` usa `>=`/`<=`: a aresta comum de 2 retângulos vizinhos fica dentro. Polígonos separados por volume não deixam fresta.

## Medição (2026-10-09, cliente Fafurion)

Sonda descartável nos 233 tiles `X_Y.unr` (evidência em `C:/Workspace/zone-builder-notes/zona-de-agua/00-evidence/`):

| Fato | Valor |
| --- | --- |
| `WaterVolume` vivos | 673, em 201 tiles; máximo por tile: 8 em 22_22, 13 em 22_24, 16 em 23_22 |
| Faces BSP por volume | 6 em todos os 673 (hexaedros convexos) |
| `Rotation` gravada | 0 volumes |
| `MainScale`/`PostScale` = (1,1,1) com `SheerRate 0` | 667; nos outros 6 a sonda não decodificou o índice compacto de 2 bytes |
| Parede inclinada (normal fora dos eixos) | 19 volumes |
| Topo inclinado | 6 volumes |
| Passam do próprio tile | 5 volumes |
| Pares que se cruzam em XY e Z com topos diferentes | 26 (12 em 25_25, 10 em 20_14) |
| Pares que se cruzam só em XY | 117 |
| Superfícies BSP de água viradas para cima | 399; das 329 que têm volume, 306 têm Z igual ao topo; 70 não têm volume |

Depois da implementação, a leitura completa do app achou 674 volumes vivos em 202 tiles (os 673 mais `17_13_classic`): 0 `Unsupported` e 13 não exatos. O leitor de `Scale` do app decodifica todos.

Esses números justificam os avisos "exata/aproximada" (paredes e topos inclinados existem) e "Água sobreposta" (há volumes com topos diferentes que se cruzam). Rotação e escala nunca aparecem nos dados reais, então só são testadas em dados sintéticos.

## Decisão: geometria

A geometria do volume são **todos** os polígonos dos nós BSP do Model referenciado pela propriedade `Brush` do ator, sem o filtro `PFNotVisible`.

- Motivo: o UE2 testa se um ponto está num volume com `PointCheck` contra o BSP do brush. Isso é inferência sobre o UE2, confirmada só indiretamente: os 129 shapes do datapack que batem com volumes batem com a caixa das faces, não com `Model.Points` (ADR 0005).
- Volume vivo: export `WaterVolume` em `Level.Actors` sem `bDeleteMe`. `bHidden` é ignorado, porque volume é invisível por natureza.
- Transformação: `mundo = Location + PostScale · R(Rotation) · MainScale · (v − PrePivot)`, sem `DrawScale`. É inferência a partir do código do UE2 (o UE2-Studio não transforma brushes); como nenhum volume real é rotacionado ou escalado, só o teste sintético `TestBrushTransformAppliesTheABrushOrder` confere a ordem. `SheerRate ≠ 0` torna o volume `Unsupported`: aparece com o motivo e não pode ser selecionado.
- `Scale` é uma lista de propriedades aninhada com tag (`Scale` Vector, `SheerRate` float, `SheerAxis` byte), lida como `TerrainLayer`, não 17 bytes crus.
- Os planos das faces são orientados pela média dos vértices do volume, não pelo sentido de giro do polígono (caso da cunha).

**Descartadas:**

- `Model.Points`: tem pontos órfãos. Em 22_22, `WaterVolume0` tem `Points` de x 69.989 a 98.304, mas as faces vão de x 86.757 a 98.304 (caixa completa das faces: x 86.757…98.304, y 152.256…153.600, z −8.778…−3.780; teste `TestGiranWaterVolumesComeFromTheBrushFaces`, pulado sem `ZB_CLIENT`).
- O export `Polys`: é a forma do editor. Lê-lo exigiria interpretar o trailer do Model até a referência (UE2-Studio `asset_bundle_native.rs:377-466`), sem ganho.
- A superfície com material de água: 70 das 399 superfícies não têm volume, água de static mesh não é BSP, e o fundo visto através da água não é água.

## Decisão: seleção

- `World.PickWater(raio, maxDist)` recorta o raio pelos planos das faces de cada volume convexo (exato). Ganha o volume com a menor entrada em `[0, maxDist]`. `maxDist` é a distância do `Pick` normal, ou o plano distante quando a cena não tem acerto. Com a câmera dentro de um volume a entrada é 0, e o clique seleciona a água em volta da câmera.
- O pick só roda no clique esquerdo sem ferramenta armada e longe das alças de vértice (o caminho em que `zones.click` devolve ""): as ferramentas de zona têm prioridade.
- Cada volume selecionado é desenhado como `render.ZoneShape` com contorno, fundo e topo passados por `ToServer`; o overlay aplica `FromServer`, então o prisma cai exatamente no volume do cliente, sem renderer novo. Cor da `zone.Water`; volumes aproximados recebem a marca `Problem`. A seleção é indexada por (tile, export), e volumes de tiles descarregados saem dela.
- **Um clique seleciona só o volume clicado; Ctrl+clique adiciona ou remove outros** (aprovado pelo usuário em 2026-10-09).

**Descartada:** selecionar o corpo d'água inteiro (volumes que se tocam com o mesmo topo, entre tiles, via `World.WaterBody`). Foi a primeira versão; no uso, desmarcar com Ctrl dava mais trabalho que marcar, e `World.WaterBody` foi removido. Não reintroduzir.

## Decisão: compilação

- Assinatura: `water.Compile(selected, live []scene.WaterVolume, doc *zone.Document) []Plan`. O aviso de sobreposição precisa dos volumes vivos fora da seleção; o plano devolve o ID da zona existente ou reserva um novo.
- Os volumes são agrupados pelo topo **já arredondado em Z do servidor**, pela regra do maior `zmax`: 1 zona por topo, 1 polígono por volume.
- Contorno: casco convexo XY dos vértices das faces, sem pontos colineares, arredondado para inteiros; uma caixa dá 4 pontos, como no datapack. Cada polígono tem `zmin = round(fundo) − 30` e `zmax = round(topo) − 30`.
- `type="water"` sem `<set>`: nenhum dos 205 shapes do `water.xml` do datapack usa parâmetro.
- O nome segue o volume (`[X_Y_<volume>]`) e evita os nomes `[X_Y_waterN]` do datapack, porque o servidor sobrescreve sem aviso uma zona de mesmo nome.

**Descartada:** unir os volumes de um topo num só polígono. Exigiria união booleana de polígonos só para economizar linhas de XML, e `Polygon.isInside` já é inclusivo.

## Fora do escopo

- Desenhar todos os volumes do mapa (o "show volumes" do UnrealEd).
- Gerar outros tipos de zona a partir de `PhysicsVolume` ou `BlockingVolume`.
