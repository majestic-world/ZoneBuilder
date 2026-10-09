# 09: Ajustar a faixa ao chão da área

**What to build:** "Recalcular pelo chão" no inspetor passa a usar o chão da área inteira do shape, não só o chão sob os vértices: o piso fica a margem configurada abaixo do chão mais baixo, e o topo a margem acima do chão mais alto. A janela de altura ganha "Piso ao chão" e "Topo ao chão", que ajustam um lado só em todos os shapes da zona, num único passo de desfazer. O terreno sempre conta; uma camada de BSP ou de mesh só puxa a faixa quando cruza a faixa atual ou fica perto dela (até 1024), para que telhados e cavernas longe não estiquem a faixa. As camadas ignoradas aparecem no inspetor como "outras camadas", com o Z delas (spec D5). A busca do chão sob cada vértice, que "Recalcular pelo chão" usava, sai.

**Blocked by:** 04

**Status:** resolved

- [x] Teste do seam: um telhado 3000 acima da faixa não puxa o topo; um mezanino 200 acima puxa
- [x] Teste do seam: o chão mais alto entre os vértices, longe deles, define o topo
- [x] No app, "Recalcular pelo chão" no retângulo da captura deixa a cobertura em 100%
- [x] "Piso ao chão" e "Topo ao chão" numa zona com vários shapes são desfeitos com um único desfazer

## Comments

Implementado na branch `cv/09-ajustar-faixa-ao-chao` e integrado em `cobertura-vertical`. Seam: telhado 3000 acima não puxa o topo e mezanino 200 acima puxa; pico longe dos vértices define o topo; uma torre não sobe de piso em piso até o telhado (revisão). Smoke: "Recalcular pelo chão" na captura deixa 100% dentro; "Piso ao chão" e "Topo ao chão" usam 1 `zone.Batch` e voltam com 1 desfazer. Exclusões mantêm a faixa delas (revisão).

Evidências: `C:/Workspace/zone-builder-notes/cobertura-vertical/09-evidence/, C:/Workspace/zone-builder-notes/cobertura-vertical/review-evidence/`.
