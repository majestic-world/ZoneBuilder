# 16: Validação e painel de problemas

**What to build:** o usuário vê os problemas de cada zona enquanto edita, num painel atualizado a cada comando. Clicar num problema seleciona a zona e leva a câmera até ela, e a zona com problema fica destacada no viewport. A compilação é bloqueada, sem gravar nada, enquanto houver problema numa zona selecionada.

O Documento expõe `Problems()` com estas regras:
- **Polígono:** 3 ou mais vértices, simples (todos os pares de arestas não adjacentes testados) e sem vértices consecutivos iguais.
- **Retângulo:** exatamente 2 cantos.
- **Faixa Z:** `zmin <= zmax`.
- **Zona:** pelo menos 1 shape incluído e tipo dentro do enum.
- **Nome:** único no projeto.
- **Parâmetros obrigatórios por tipo:** `residence` em SIEGE e HEADQUARTER; `distribution_id` e `fishing_place_type` em FISHING; nome `residence_<id>` em RESIDENCE.
- **Coordenadas:** dentro de X ∈ [−163840, 229375] e Y ∈ [−262144, 294911].

**Blocked by:** 13 (Retângulo, círculo, exclusões e restart points), 14 (Painel de propriedades), 15 (Lista de zonas)

**Status:** ready-for-agent

- [ ] Cada regra aparece no painel, com zona, shape ou vértice indicado
- [ ] Clicar num problema seleciona a zona, destaca o vértice ou shape e leva a câmera até ele
- [ ] A lista de zonas mostra a contagem de problemas por zona
- [ ] Teste do seam Documento: auto-interseção na aresta 0→1 e vértices consecutivos iguais geram problema
- [ ] Teste do seam Documento: uma zona sem shape incluído gera problema
- [ ] Teste do seam Documento: um nome duplicado no projeto gera problema
- [ ] Teste do seam Documento: com uma zona selecionada inválida, a compilação falha sem gravar arquivo
