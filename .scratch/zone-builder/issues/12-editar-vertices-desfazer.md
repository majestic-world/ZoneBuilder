# 12: Editar vértices, faixa Z e desfazer

**What to build:** o usuário corrige uma zona sem redesenhar.
- **Vértices:** arrastar (o vértice volta a encostar na superfície sob o cursor), inserir no meio de uma aresta, apagar e editar as coordenadas pelo teclado.
- **Shape:** mover o shape inteiro.
- **Faixa Z:** editar `zmin` e `zmax` de cada shape, recalcular a faixa a partir do chão sob os vértices e configurar a folga padrão.
- **Desfazer e refazer:** cobrem toda edição, porque toda edição é um comando do Documento.

**Blocked by:** 05 (Primeira zona de ponta a ponta)

**Status:** resolved

- [x] Arrastar um vértice sobre um telhado ou piso o reposiciona no ponto atingido
- [x] Inserir, apagar e mover vértices e shapes se reflete no prisma e no XML compilado
- [x] Recalcular a faixa Z depois de mover o contorno usa o chão sob os novos vértices e a folga configurada
- [x] Desfazer volta cada passo e refazer reaplica
- [x] Teste do seam Documento: uma sequência de comandos seguida do mesmo número de `Undo` devolve o documento inicial, e `Redo` reaplica

## Comments

Resolvido na branch de integração `zone-builder`. Branch `zb/12-editar-vertices-desfazer`. Undo/redo por snapshot em `Document.Apply`, cobrindo todos os comandos. Teste de N comandos + N Undo + Redo.
