# Zona de água: `zmin` e `zmax` ficam 30 abaixo do `WaterVolume`, contra o +32 do ADR 0003

A zona `water` sai do `WaterVolume` do cliente (`internal/water`; a fonte da geometria está no [ADR 0011](0011-agua-do-brush-do-watervolume.md)). O `zmax` dela **é** a superfície da água para o servidor: decide quando o personagem nada, quando perde fôlego e onde a boia de pesca fica. Para passar o Z do volume ao servidor há 2 dados, e eles se contradizem:

| Fonte | Topo da zona em relação ao topo do volume |
| --- | --- |
| [ADR 0003](0003-z-do-servidor-32-acima-do-cliente.md): chão do servidor = chão do cliente + 32 | +32 |
| `water.xml` do datapack Majestic, gerado dos volumes | −30 |

## Medição (2026-10-09, cliente Fafurion, datapack Majestic)

Uma sonda descartável leu os `WaterVolume` vivos dos 233 tiles `X_Y.unr` e comparou cada shape de `gameserver/data/zone/water.xml` com a caixa das faces BSP dos volumes do mesmo tile (caixa XY a ±2).

| Fato | Valor |
| --- | --- |
| `WaterVolume` vivos | 673, em 201 tiles |
| Shapes do `water.xml` que batem com um volume | 129 de 205 |
| `zmin` e `zmax` do datapack menos o do volume, nesses 129 | os 2 em −30 (±0,05) em 119 |
| Só o `zmax` em −30 | mais 5 |
| Os 10 restantes | ajustes à mão (−46, −22, −1, +2970…) |
| Shapes que não batem | 76: 15 são o tile inteiro, o resto é ajuste à mão ou outra versão do mapa |

Os shapes que batem são a caixa XY das faces do volume (não a de `Model.Points`) com `zmin`/`zmax` = Z do cliente − 30, 1 zona por volume. Em 22_24, `[22_24_water1…9]`, `[22_24_water12]` e `[22_24_water13]` são 11 dos 13 volumes; a fonte (`WaterVolume26` e `WaterVolume27`) foi ajustada à mão em `[22_24_water10]` e `[22_24_water11]`. O teste `TestFloranLakeReproducesTheDatapack` copia esses 11 shapes e confere que o app gera os mesmos pontos e o mesmo `zmin`/`zmax`.

A sonda e a saída estão em `zone-builder-notes/zona-de-agua/00-evidence/`, fora do repositório, porque leem o datapack.

## Decisão

`water.ServerZOffset = -30`: `zmin = round(fundo) − 30` e `zmax = round(topo) − 30`, numa constante única, separada do `scene.ServerZOffset = 32`.

- A zona compilada se comporta como as cerca de 200 zonas de água que o servidor já carrega: um lago feito no app nada e respira igual ao mar ao lado.
- O +32 do ADR 0003 mede o chão da geodata, não a água. O servidor não lê a água da geodata; ele só tem a zona. Seguir o +32 deixaria a superfície 62 acima da das zonas do datapack.
- **Pendente:** falta medir em jogo onde o personagem começa a nadar e a perder fôlego (ver [Pendente](#pendente)). Até lá, −30 vale por ser o que o servidor já usa.

**Descartada:** +32, pela regra do ADR 0003. É coerente com o resto do app, mas diverge de 119 dos 129 shapes do datapack, que é o que o servidor carrega hoje.

## Consequences

- **A zona compilada aparece no viewport com o topo 62 unidades abaixo da superfície da água.** O overlay converte do servidor com `FromServer` (−32, ADR 0003), então o topo `T − 30` é desenhado em `T − 62`. É esperado: não "corrija" o overlay nem a constante sem a medição em jogo (ver [Pendente](#pendente)). A seleção da água continua desenhando o volume do cliente, sem esse deslocamento.
- O `zmin` e o `zmax` de uma zona de água compilada não seguem a regra "Z do servidor = Z do cliente + 32" que vale para as outras zonas e para o `Hit` do `Pick`. Quem converter um Z de água deve usar `water.ServerZOffset`, nunca `scene.ToServer`.
- Se a medição trocar a constante, as zonas de água já salvas nos `.zbproj` ficam com o valor antigo: o app nunca recompila uma zona cujo nome já existe no projeto.

## Pendente

`water.ServerZOffset = -30` ainda não foi verificado em jogo. Procedimento:

1. Pôr no datapack a zona de um lago compilada pelo app. Usar um lago sem zona no `water.xml`, ou trocar a zona de 22_22 pela do app depois de remover a antiga.
2. Reiniciar o servidor. Como GM:
   1. Andar da margem para dentro da água. Anotar o `//pos` quando o personagem começa a nadar no cliente e quando o servidor aplica a velocidade de nado.
   2. Mergulhar até aparecer a barra de fôlego e anotar o `//pos`.
   3. Pescar da margem, com uma zona `FISHING` por cima, e conferir que a boia fica na superfície.
3. Registrar as 3 medições com `//pos` e o topo do volume.

Se o nado do cliente e o do servidor diferirem em mais de 16 unidades, reabrir este ADR com a medição e trocar `water.ServerZOffset`. Se a constante mudar, atualizar também `docs/uso.md` §Zona de água ("Z = Z do cliente − 30" e a nota dos "62 abaixo") e o termo "Topo" do `GLOSSARY.md`.
