# 08: Gerar, pinos e ajuste de pontos

**What to build:** o painel da área ganha **Gerar** e **Regerar** (outra semente). A geração roda fora do laço de eventos sobre um snapshot do `World`, com o aviso de carga, usando `placement.Distribute` com a altura do monstro de prévia, e volta como um único `SetPoints` (spec D3). Regerar avisa que descarta os ajustes manuais e pode ser desfeito. Cada ponto aparece no viewport como pino com o círculo do raio no chão; pontos desatualizados aparecem diferentes. O usuário arrasta um ponto (ele cai no chão sob o cursor), apaga e adiciona pontos com clique, tudo com desfazer. O inspetor mostra a área de chão livre, o espaçamento médio e o menor espaçamento; os avisos "cabem K de N" e "nenhuma célula livre" aparecem sem bloquear. Contagens flexionadas nos 2 idiomas.

**Blocked by:** 06, 07

**Status:** ready-for-agent

- [ ] Smoke no app: área de 50 numa clareira com árvores, pedra e cercas gera pontos visíveis como pinos com círculo, nenhum sobre mesh, água ou a menos de raio + afastamento de tronco, cerca ou pedra (conferido nos screenshots e por um dump dos pontos)
- [ ] Smoke no app: gerar 2 vezes com a mesma semente dá os mesmos pontos; Regerar dá outros, avisa sobre ajustes manuais e é desfeito por 1 `Undo`
- [ ] Smoke no app: arrastar, apagar e adicionar ponto, com desfazer; o ponto arrastado cai no chão sob o cursor
- [ ] Smoke no app: mudar a quantidade depois de gerar deixa os pinos com o visual de desatualizado e o problema no painel
- [ ] Smoke no app: área pequena mostra "cabem K de N"; área sobre água mostra "nenhuma célula livre"; o inspetor mostra os 3 números
- [ ] A interface continua fluida durante a geração
