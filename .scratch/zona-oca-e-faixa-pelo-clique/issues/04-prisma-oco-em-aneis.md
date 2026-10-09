# 04: Prisma oco em anéis

**What to build:** o overlay deixa de preencher o prisma (spec D4). As tampas saem, junto com `triangulate`. As paredes continuam como triângulos, com um atributo novo, o `zmin` do shape em Z do cliente, e são desenhadas por um modo novo, `modeWall`, que só pinta anéis horizontais:

- a cada passo a partir do `zmin`, com 1 px e antialias por `fwidth`;
- passo-base de 64, que dobra até os anéis ficarem a pelo menos 6 px um do outro, com esmaecimento entre os níveis;
- alfa 0,55 à frente da cena (`GEQUAL`) e 35% disso atrás dela (`LESS`).

`modeBuriedFace` e `prismAlpha` saem. As linhas (contornos, arestas verticais e linha do chão) não mudam. Vale para zonas, exclusões, a prévia de desenho e os volumes de água.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] No app, no 23_18, com a câmera dentro da `[Baium_PvP_Zone]`: o mapa aparece sem tinta nem hachura do overlay, com os anéis nas paredes ao redor
- [x] Vista de fora da torre: os anéis leem o volume; de longe não viram uma parede sólida, e de perto mostram a grade fina
- [x] Parede vista quase de perfil: os anéis não cintilam ao mover a câmera
- [x] Parte enterrada (zona do morro do 22_22): os anéis fracos e as arestas tracejadas aparecem atrás do terreno
- [x] Os volumes de água do 22_22 aparecem no mesmo estilo
- [x] O FPS não cai com a torre inteira na tela (`-fps` antes e depois)

## Comments

Implementado na branch `zo/04-prisma-oco-em-aneis` e integrado em `zona-oca-e-faixa-pelo-clique`. A mudança ficou só em `internal/render/overlay.go`:
- saíram as tampas e o `triangulate`;
- novo atributo `overlayVertex.Base`, com o `zmin` do shape;
- `modeWall`/`modeBuriedWall` com passo 64 que dobra até ≥ 6 px, com esmaecimento;
- alfa 0,55 à frente da cena e 35% disso atrás dela;
- saíram `modeBuriedFace` e `prismAlpha`.

Capturas antes e depois:
- dentro da `[Baium_PvP_Zone]`;
- de fora, de perto e de longe;
- parede quase de perfil, em sequências de quadros parados;
- parte enterrada do morro do 22_22;
- água do 22_22.

FPS antes e depois:

| Vista | Antes | Depois |
| --- | --- | --- |
| De longe | 92,1–92,9 | 92,4–93,2 |
| Meio | 91,7–93,2 | 91,5–92,4 |
| Dentro | 92,9–94,8 | 94,6–95,1 |

A cintilação foi conferida em quadros parados, não com a câmera em movimento. A prévia de desenho não foi verificada à parte, porque passa pelo mesmo `set()`.

Evidências: `C:/Workspace/zone-builder-notes/zona-oca-e-faixa-pelo-clique/04-evidence/`.
