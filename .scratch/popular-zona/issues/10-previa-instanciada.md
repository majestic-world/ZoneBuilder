# 10: Prévia com o monstro instanciado

**What to build:** um botão flutuante "Prévia" no viewport do modo população troca os pinos pelo death knight wizard embutido em cada ponto, em idle (`Wait`), texturizado, com os pés no chão e virado para o heading que vai para o servidor (spec D7). Os bindings GLES ganham `glDrawElementsInstanced` e `glVertexAttribDivisor`; um passe novo, entre Masked e Translucent, desenha um modelo com N instâncias: o VBO de vértices sobe com o skinning da CPU do frame (uma pose para todas as instâncias), cada instância passa `x y z yaw` por atributo com divisor 1 e o shader converte do servidor para a cena (`FromServer`, rebase). Shader texturizado com corte em alfa 0,5 nas seções masked, luz fixa `0,60 + 0,40·max(N·L, 0)` e depth com Z reverso (ADR 0001). O heading do L2 vira yaw na mesma unidade (65536 = 360°) **[INFERENCE]**.

**Blocked by:** 05, 08

**Status:** resolved

- [x] Smoke no app: com a prévia ligada, cada ponto mostra o monstro em `Wait`, texturizado, com os pés no chão e o cajado na mão (screenshot)
- [x] Smoke no app: girar o heading de um ponto (ou gerar outro) gira o monstro coerente com o heading
- [x] Smoke no app: 500 instâncias mantêm 60 fps (medido com `-fps`)
- [x] Desligar a prévia volta aos pinos; o modo de zonas não muda

## Comments

Integrado em `popular-zona`, a partir de `pz/10-previa-instanciada` (`c7aab1c`). Smoke: Wait animado e texturizado, cajado na mão, direção comparada com linhas de heading, retorno aos pinos e alternância de modos. Medição com 500 instâncias: aproximadamente 142–146 fps, pior quadro de cerca de 10 ms. Build, vet e testes passaram com `ZB_CLIENT`. Retomada: prévia e humano vistos juntos no smoke da branch integrada.

Render API e evidências: `C:/Workspace/zone-builder-notes/popular-zona/10-previa-instanciada.md` e `10-evidence/`. A correspondência do heading no cliente do jogo continua pendente no ticket 13; não é confirmada pelo smoke da prévia.
