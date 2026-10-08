# 13: Retângulo, círculo, exclusões e restart points

**What to build:** o usuário usa os outros shapes e elementos de zona, todos chegando ao XML compilado.
- **Retângulo:** 2 cliques em cantos opostos.
- **Círculo:** centro e raio, convertido em polígono de N lados antes de chegar ao Documento, porque o `circle` do servidor se comporta como quadrado.
- **Vários shapes incluídos** na mesma zona.
- **Exclusões:** `banned_polygon` criadas com as mesmas ferramentas dentro da zona selecionada.
- **Pontos de restart:** `restart_point` e `PKrestart_point` marcados com cliques.

**Blocked by:** 05 (Primeira zona de ponta a ponta)

**Status:** resolved

- [x] Um retângulo compila como `rectangle` com 2 coords de 4 números
- [x] Um círculo compila como `polygon`, e nenhum XML contém `circle`
- [x] Uma zona com 2 polígonos incluídos e 1 exclusão compila com a exclusão como `banned_polygon`
- [x] `restart_point` e `PKrestart_point` compilam com `x y z` dos cliques
- [x] No servidor Java sem alteração, `//zone_check` responde conforme o desenho dentro da zona, fora dela e dentro da exclusão

## Comments

Resolvido na branch de integração `zone-builder`. Branch `zb/13-shapes-exclusoes-restart`. Retângulo, círculo como polígono de 32 lados, exclusões como `banned_polygon` e restart points. Sondas do harness (dentro, fora, dentro da exclusão) conforme o desenho. Manual: `//zone_check` no jogo.
