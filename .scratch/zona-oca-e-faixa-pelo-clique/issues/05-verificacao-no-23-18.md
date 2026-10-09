# 05: Verificação no 23_18

**What to build:** smoke dos 2 pontos juntos, no cliente real, com capturas de tela, repetindo o cenário do relato: abrir o 23_18, voar até a torre, criar a zona no topo nos 2 modos da faixa nova, entrar nela com a câmera e conferir números, régua, avisos e a vista interna. Arrastar a seta Z com `-fps` ligado.

**Blocked by:** 01, 02, 03, 04

**Status:** needs-triage

- [ ] Modo chão da área: o polígono no topo nasce perto do topo; a régua mostra o terreno como outra camada; nenhum aviso de chão abaixo do piso
- [ ] Modo pontos clicados: o polígono nasce com `vértices ± folga`
- [ ] A zona da captura (−5376 … 10453) corrigida com Base, Altura e "Piso ao chão" fica no topo
- [ ] A vista de dentro da zona fica limpa, como no jogo: anéis nas paredes, mapa visível
- [ ] `Classify` com p99 abaixo de 2 ms ao arrastar a seta Z
- [ ] Capturas antes e depois guardadas nas evidências

## Comments
