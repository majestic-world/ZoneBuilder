# 05: Verificação no 23_18

**What to build:** smoke dos 2 pontos juntos, no cliente real, com capturas de tela, repetindo o cenário do relato: abrir o 23_18, voar até a torre, criar a zona no topo nos 2 modos da faixa nova, entrar nela com a câmera e conferir números, régua, avisos e a vista interna. Arrastar a seta Z com `-fps` ligado.

**Blocked by:** 01, 02, 03, 04

**Status:** resolved

- [x] Modo chão da área: o polígono no topo nasce perto do topo; a régua mostra o terreno como outra camada; nenhum aviso de chão abaixo do piso
- [x] Modo pontos clicados: o polígono nasce com `vértices ± folga`
- [x] A zona da captura (−5376 … 10453) corrigida com Base, Altura e "Piso ao chão" fica no topo
- [x] A vista de dentro da zona fica limpa, como no jogo: anéis nas paredes, mapa visível
- [x] `Classify` com p99 abaixo de 2 ms ao arrastar a seta Z
- [x] Capturas antes e depois guardadas nas evidências

## Comments

Smoke integrado na branch `zona-oca-e-faixa-pelo-clique` (`2ebb922`), com o cliente real. Para as capturas "antes", o `main` (`62736a0`) foi compilado à parte. Nenhum defeito precisou de código.

**Modo chão da área.** O polígono no topo da torre nascia com −5376 … 10340 e agora nasce com 8464 … 10340. O terreno (−5120) sai da régua e aparece em primeiro lugar em "Outras camadas", e nenhum aviso vem dele.

Sobra 1 aviso de chão abaixo do piso, com folga −990, que vem de um piso interno de BSP da torre a z 7474:
- o `Fit` julga as camadas a partir da faixa dos pontos clicados;
- o `Classify` julga de novo a partir da faixa ajustada e alcança mais um andar.

É a cascata do ADR 0004 ("quem decide é o usuário") e o risco aceito na spec ("pisos internos da torre"). A saída é o modo pontos clicados. O ticket 06 documenta isso.

**Modo pontos clicados.** O polígono nasce com `z 9827..10340 pelos pontos clicados`, que é 10083 ∓ 256 (folga 256).

**Zona do relato.** Com Base 9941, Altura 512 e depois "Piso ao chão", a faixa fica em 8512 … 10453. Antes da mudança, ela voltava para −5376 … 10453.

**Vista de dentro** (pose 114750, 16125, 10350): antes, a tela inteira ficava verde e hachurada. Agora o mapa aparece limpo, com anéis nas paredes, e a pegada fica só no piso.

**`Classify`, p99 em ms, antes → depois:**

| Caso | Antes | Depois |
| --- | --- | --- |
| Zona do relato no 23_18 | 0,088 | 0,092 |
| Tile inteiro do 23_18 | 1,760 | 1,822 |
| Morro do 22_22 | 0,022 | 0,022 |
| Tile inteiro do 22_22 | 1,587 | 1,600 |

Cada caso tem de 320 a 960 amostras, medidas com instrumentação QPC descartável.

Evidências: `C:/Workspace/zone-builder-notes/zona-oca-e-faixa-pelo-clique/05-evidence/` e `C:/Workspace/zone-builder-notes/zona-oca-e-faixa-pelo-clique/05-verificacao-no-23-18.md`.
