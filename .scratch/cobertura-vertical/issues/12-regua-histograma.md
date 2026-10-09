# 12: Régua com histograma na janela de altura

**What to build:** a janela de altura ganha uma régua vertical, que se lê sem depender do ângulo da câmera: o eixo Z vai do menor entre `zmin` e o chão mais baixo ao maior entre `zmax` e o chão mais alto; a faixa da zona aparece como uma barra; ao lado, um histograma da área de chão por Z, colorido pelos 3 estados (dentro, acima, abaixo); e marcas no chão mais baixo e no mais alto com as folgas escritas (spec D7). A régua é só leitura e acompanha o arrasto da seta Z.

**Blocked by:** 06

**Status:** ready-for-agent

- [ ] Captura de tela da régua para a zona da captura, antes e depois de "Recalcular pelo chão"
- [ ] Arrastar a seta Z move a barra da faixa e recolore o histograma a cada frame
