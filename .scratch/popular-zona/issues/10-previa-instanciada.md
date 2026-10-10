# 10: Prévia com o monstro instanciado

**What to build:** um botão flutuante "Prévia" no viewport do modo população troca os pinos pelo death knight wizard embutido em cada ponto, em idle (`Wait`), texturizado, com os pés no chão e virado para o heading que vai para o servidor (spec D7). Os bindings GLES ganham `glDrawElementsInstanced` e `glVertexAttribDivisor`; um passe novo, entre Masked e Translucent, desenha um modelo com N instâncias: o VBO de vértices sobe com o skinning da CPU do frame (uma pose para todas as instâncias), cada instância passa `x y z yaw` por atributo com divisor 1 e o shader converte do servidor para a cena (`FromServer`, rebase). Shader texturizado com corte em alfa 0,5 nas seções masked, luz fixa `0,60 + 0,40·max(N·L, 0)` e depth com Z reverso (ADR 0001). O heading do L2 vira yaw na mesma unidade (65536 = 360°) **[INFERENCE]**.

**Blocked by:** 05, 08

**Status:** ready-for-agent

- [ ] Smoke no app: com a prévia ligada, cada ponto mostra o monstro em `Wait`, texturizado, com os pés no chão e o cajado na mão (screenshot)
- [ ] Smoke no app: girar o heading de um ponto (ou gerar outro) gira o monstro coerente com o heading
- [ ] Smoke no app: 500 instâncias mantêm 60 fps (medido com `-fps`)
- [ ] Desligar a prévia volta aos pinos; o modo de zonas não muda
