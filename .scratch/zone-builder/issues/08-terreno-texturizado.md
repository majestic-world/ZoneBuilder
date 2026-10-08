# 08: Terreno texturizado

**What to build:** o terreno aparece texturizado como no jogo.
- **Camadas:** a camada 0 é opaca e as seguintes são misturadas em blend alfa pela máscara do canal R do alpha map, com profundidade `>=` sem escrita.
- **UV por camada:** com rotação, pan e escala.
- **Texturas:** o port da leitura de mips e da decodificação DXT1, DXT3, DXT5 e RGBA8 (gravado como BGRA), seguindo a decisão do ADR de texturas do ticket 01.
- **Material:** inclui o bloco nativo antes das mips, com as faixas de licensee.
- **Amostragem:** mipmaps, repeat e filtro anisotrópico.

**Blocked by:** 03 (Terreno sem textura no viewport)

**Status:** ready-for-agent

- [ ] O terreno de Giran mostra caminhos, gramados e transições entre camadas iguais ao modo Textured do UE2-Studio na mesma posição de câmera
- [ ] Uma camada sem alpha map é desenhada inteira, como no UE2-Studio
- [ ] Teste do seam Cena: blocos DXT1/3/5 reais do cliente decodificam para os pixels esperados, pegando troca de canal BGRA e o alfa 1-bit do DXT1
