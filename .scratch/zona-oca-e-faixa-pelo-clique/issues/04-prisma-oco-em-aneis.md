# 04: Prisma oco em anéis

**What to build:** o overlay deixa de preencher o prisma (spec D4). As tampas saem, junto com `triangulate`. As paredes continuam como triângulos, com um atributo novo, o `zmin` do shape em Z do cliente, e são desenhadas por um modo novo, `modeWall`, que só pinta anéis horizontais:

- a cada passo a partir do `zmin`, com 1 px e antialias por `fwidth`;
- passo-base de 64, que dobra até os anéis ficarem a pelo menos 6 px um do outro, com esmaecimento entre os níveis;
- alfa 0,55 à frente da cena (`GEQUAL`) e 35% disso atrás dela (`LESS`).

`modeBuriedFace` e `prismAlpha` saem. As linhas (contornos, arestas verticais e linha do chão) não mudam. Vale para zonas, exclusões, a prévia de desenho e os volumes de água.

**Blocked by:** None (can start immediately)

**Status:** needs-triage

- [ ] No app, no 23_18, com a câmera dentro da `[Baium_PvP_Zone]`: o mapa aparece sem tinta nem hachura do overlay, com os anéis nas paredes ao redor
- [ ] Vista de fora da torre: os anéis leem o volume; de longe não viram uma parede sólida, e de perto mostram a grade fina
- [ ] Parede vista quase de perfil: os anéis não cintilam ao mover a câmera
- [ ] Parte enterrada (zona do morro do 22_22): os anéis fracos e as arestas tracejadas aparecem atrás do terreno
- [ ] Os volumes de água do 22_22 aparecem no mesmo estilo
- [ ] O FPS não cai com a torre inteira na tela (`-fps` antes e depois)

## Comments
