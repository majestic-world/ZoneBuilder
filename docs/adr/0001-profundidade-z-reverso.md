# Profundidade: Z reverso com `GL_EXT_clip_control` e Depth32F num framebuffer do viewport

O UE2-Studio desenha com Z reverso (Depth32Float, limpo em 0, teste `Greater`/`GreaterEqual`, near plane de 1 unidade) para manter camadas de chão a menos de 1 unidade separadas mesmo vendo boa parte do mapa. O GLES 3.0 não tem `glClipControl`, mas o ANGLE expõe `GL_EXT_clip_control` e exporta `glClipControlEXT`: medido nesta máquina (RTX 4070 Ti SUPER, backend Direct3D11) nos 3 builds de ANGLE disponíveis (2.1.26625 `a96fca8d5ee2` do JCEF do PhpStorm, 2.1.23105 `5d4df51d1d7d` do CEF da Steam, 2.1.23531 `713102774487` do NVIDIA App). Por isso o Zone Builder usa o mesmo esquema do UE2-Studio: `glClipControlEXT(LOWER_LEFT, ZERO_TO_ONE)`, projeção que leva o near plane a 1 e o far plane a 0, profundidade limpa em 0 e teste `GREATER` (`GEQUAL` para o TerrainLayer). O app recusa iniciar se o ANGLE não tiver a extensão.

Z reverso só ganha precisão com profundidade em ponto flutuante, e a superfície de janela do EGL não tem profundidade float. Por isso o viewport desenha num framebuffer próprio, do tamanho do widget, com cor `SRGB8_ALPHA8` e profundidade `DEPTH_COMPONENT32F`. Depois um passe de shader copia a cor para o retângulo do viewport na janela, e o Gio desenha os painéis por cima. A cópia fica toda na GPU, sem a leitura para a CPU que o plano descartou (`paint.ImageOp` por frame).

## Considered Options

- **Profundidade padrão com near plane maior, ou depth logarítmico no shader** (as alternativas do plano). Desnecessárias, porque a extensão existe. O depth logarítmico ainda escreve `gl_FragDepth`, o que desliga o early-Z.
- **Desenhar direto na janela com `glViewport`/`glScissor`** (o desenho original do plano). Ficaria preso à profundidade de ponto fixo da superfície EGL, o que anula o Z reverso.
- **`glBlitFramebuffer` em vez do passe de shader.** Este ANGLE não oferece `EGL_KHR_gl_colorspace`, então a janela é linear. Um blit de origem sRGB para destino linear lineariza as cores (GLES 3.0 §4.3.3) e escurece a imagem. O shader reaplica a codificação sRGB quando a janela é linear.

## Consequences

- O `glClipControlEXT` volta para `NEGATIVE_ONE_TO_ONE` e o teste de profundidade é desligado antes do frame do Gio, que compartilha o contexto (`gpu.OpenGL{Shared: true}`).
- Conferido no spike: o cubo é desenhado sem face culling, então só o teste de profundidade esconde as faces de trás. Trocar temporariamente para `LESS` com profundidade limpa em 1 mostra o interior do cubo, o que confirma que o mapeamento reverso está ativo.
- MSAA (o UE2-Studio usa 4x) exige renderbuffers multisample no framebuffer do viewport e um resolve por blit para uma textura sRGB do mesmo formato antes do passe de cópia. O resolve não pode ir direto para a janela, porque o resolve por blit exige formatos idênticos.
