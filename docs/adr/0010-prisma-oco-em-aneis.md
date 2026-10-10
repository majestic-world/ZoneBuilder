# Prisma oco em anéis, e pegada só no chão

O overlay desenhava os prismas das zonas, das exclusões, da prévia de desenho e do volume de água como sólidos: paredes e as 2 tampas triangulados, com alfa 0,22, e um segundo passe LESS que hachurava na tela tudo o que ficava atrás da cena. Com a câmera dentro de um prisma (por exemplo `[Baium_PvP_Zone]` no alto da torre do 23_18), paredes e tampas cobriam a tela inteira, e tudo saía verde e hachurado. No jogo, as zonas são desenhadas ocas, como uma parede de linhas empilhadas.

## Decisão 1: prisma oco, um estilo só

O prisma não tem tampas nem preenchimento: só anéis horizontais nas paredes, mais os contornos, as arestas verticais e a linha do chão ao longo das paredes. Paredes e linhas só aparecem na frente da cena (`GEQUAL`, [ADR 0001](0001-profundidade-z-reverso.md)); a parte enterrada não aparece.

Há **um estilo só**, sem opção "sólido". Se o volume ficar difícil de ler de fora, ajusta-se o alfa dos anéis (`wallAlpha = 0.55`) ou o passo base (`ringStep = 64`), nunca se volta o preenchimento.

**Descartada:** um preenchimento de cerca de 4% de alfa atrás dos anéis.

## Decisão 2: anéis no shader, não linhas na CPU

O passo dos anéis depende da distância da câmera. Anéis gerados como linhas na CPU exigiriam refazer o buffer a cada frame, ou um passo fixo, denso demais de longe e esparso demais de perto. O shader resolve por pixel, e o buffer de vértices só muda quando o documento muda.

Em `internal/render/overlay.go`:

- cada vértice de parede leva o `zmin` do shape em Z do cliente (`overlayVertex.Base`, atributo 3), e o shader usa `vRise = z − zmin`. O +32 do [ADR 0003](0003-z-do-servidor-32-acima-do-cliente.md) se cancela na diferença, então os anéis caem nas mesmas alturas de Z do servidor;
- um anel é `|fract(g − 0.5) − 0.5| / fwidth`: linha de 1 px com antialiasing, o mesmo padrão do `gridLine`;
- nível de detalhe: `lod = max(log2(2·ringGap·fwidth(vRise)/ringStep), 0)` e passo `64·2^floor(lod)`. O nível mais fino some enquanto os anéis dele se aproximam de 2·ringGap a ringGap px, então o passo nunca salta;
- constantes: `ringStep = 64`, `ringGap = 6` px, `wallAlpha = 0.55`; contornos opacos.

## Decisão 3: pegada só no chão visto de cima

Em `ground()` (`internal/render/ground.go`), a tinta, as hachuras de acima/abaixo e as linhas de nível são multiplicadas por `floor_ = up * step(vPos.z, uEye.z)`, com `up = smoothstep(0.3, 0.5, abs(n.z))`.

- A normal da face vem de `cross(dFdx(vPos), dFdy(vPos))`, cujo sinal não é confiável. Por isso se usa `abs(n.z)`, e o que separa chão de teto é ser visto de cima (Z do olho ≥ Z do fragmento), sem normal de vértice.
- O contorno da zona (`edge`) **não** é multiplicado por esse fator: ele marca onde a parede do prisma encontra o mapa.
- Rampas mais íngremes que cerca de 60° (`|n.z| < 0,5`, sumindo de todo em 0,3) ficam sem pegada. É o mesmo limiar que o `up` já usava para a grade.

## Consequences

- Sem custo de frame, medido com `-fps` com a torre inteira na tela, antes → depois: de longe 92,1–92,9 → 92,4–93,2; a meia distância 91,7–93,2 → 91,5–92,4; de dentro 92,9–94,8 → 94,6–95,1.
- **Não verificado:** a cintilação dos anéis numa parede vista quase de lado foi conferida só em sequências de frames parados, não com a câmera em movimento. A prévia de desenho (retângulo, círculo, polígono fechado) não foi conferida à parte no estilo de anéis; supõe-se que funciona por passar pelo mesmo `zoneOverlay.set()`.
- **Fora do escopo:** anéis no chão e tampas em grade no topo.
