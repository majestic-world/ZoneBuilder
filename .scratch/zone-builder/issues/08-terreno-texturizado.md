# 08: Terreno texturizado

**What to build:** o terreno aparece texturizado como no jogo.
- **Camadas:** a camada 0 é opaca e as seguintes são misturadas em blend alfa pela máscara do canal R do alpha map, com profundidade `>=` sem escrita.
- **UV por camada:** com rotação, pan e escala.
- **Texturas:** o port da leitura de mips e da decodificação DXT1, DXT3, DXT5 e RGBA8 (gravado como BGRA), seguindo a decisão do ADR de texturas do ticket 01.
- **Material:** inclui o bloco nativo antes das mips, com as faixas de licensee.
- **Amostragem:** mipmaps, repeat e filtro anisotrópico.

**Blocked by:** 03 (Terreno sem textura no viewport)

**Status:** resolved

- [x] O terreno de Giran mostra caminhos, gramados e transições entre camadas iguais ao modo Textured do UE2-Studio na mesma posição de câmera
- [x] Uma camada sem alpha map é desenhada inteira, como no UE2-Studio
- [x] Teste do seam Cena: blocos DXT1/3/5 reais do cliente decodificam para os pixels esperados, pegando troca de canal BGRA e o alfa 1-bit do DXT1

## Comments

Resolvido na branch de integração `zone-builder`. Branch `zb/08-terreno-texturizado`. Comparação com render offscreen do próprio UE2-Studio na mesma pose (diferença média de 2,7 a 5,6/255). Teste de DXT1/3/5 e RGBA8 com bytes reais. Em aberto: unidade de `TextureRotation` (graus, como no UE2-Studio) só se confirma no jogo.
