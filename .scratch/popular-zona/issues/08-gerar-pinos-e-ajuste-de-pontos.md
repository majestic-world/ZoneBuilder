# 08: Gerar, pinos e ajuste de pontos

**What to build:** o painel da área ganha **Gerar** e **Regerar** (outra semente). A geração roda fora do laço de eventos sobre um snapshot do `World`, com o aviso de carga, usando `placement.Distribute` com a altura do monstro de prévia, e volta como um único `SetPoints` (spec D3). Regerar avisa que descarta os ajustes manuais e pode ser desfeito. Cada ponto aparece no viewport como pino com o círculo do raio no chão; pontos desatualizados aparecem diferentes. O usuário arrasta um ponto (ele cai no chão sob o cursor), apaga e adiciona pontos com clique, tudo com desfazer. O inspetor mostra a área de chão livre, o espaçamento médio e o menor espaçamento; os avisos "cabem K de N" e "nenhuma célula livre" aparecem sem bloquear. Contagens flexionadas nos 2 idiomas.

**Blocked by:** 06, 07

**Status:** resolved

- [x] Smoke no app: área de 50 numa clareira com árvores, pedra e cercas gera pontos visíveis como pinos com círculo, nenhum sobre mesh, água ou a menos de raio + afastamento de tronco, cerca ou pedra (conferido nos screenshots e por um dump dos pontos)
- [x] Smoke no app: gerar 2 vezes com a mesma semente dá os mesmos pontos; Regerar dá outros, avisa sobre ajustes manuais e é desfeito por 1 `Undo`
- [x] Smoke no app: arrastar, apagar e adicionar ponto, com desfazer; o ponto arrastado cai no chão sob o cursor
- [x] Smoke no app: mudar a quantidade depois de gerar deixa os pinos com o visual de desatualizado e o problema no painel
- [x] Smoke no app: área pequena mostra "cabem K de N"; área sobre água mostra "nenhuma célula livre"; o inspetor mostra os 3 números
- [x] A interface continua fluida durante a geração

## Comments

Integrado em `popular-zona`, a partir de `pz/08-gerar-pinos-e-ajuste-de-pontos` (`e75b02e`). Clareira do 22_22: 50 pontos, nenhum inválido pelo oráculo independente; mesh mais próxima a 41,5 para afastamento exigido de 41. Mesma semente produz projeto idêntico; edição manual, Undo/Redo, pontos desatualizados e avisos conferidos em pt-BR/en. A geração força meshes como obstáculos mesmo quando ocultas pelo toggle do viewport.

Revisão integrada: Regerar mostra confirmação antes de alterar a semente/pontos; Cancelar preservou o projeto byte a byte e 1 Undo restaurou o baseline manual byte a byte. Chão livre de 369.920 u² persistiu após salvar, fechar e reabrir sem gerar novamente. Smoke e capturas em `review-evidence/`.

Risco medido no smoke original: geração de 8.000 pontos em segundo plano manteve aproximadamente 97–105 fps, mas o recebimento desse resultado produziu 1 quadro de cerca de 340 ms. A geração com 50 pontos foi de poucos milissegundos.

Evidências: `C:/Workspace/zone-builder-notes/popular-zona/08-gerar-pinos-e-ajuste-de-pontos.md`, `08-evidence/`, `review-fixes.md` e `review-evidence/`. Build, vet e suíte com `ZB_CLIENT` passaram no gate final.
