# 01: Spike de renderização Gio + ANGLE

**What to build:** a janela do Zone Builder abre com um widget de viewport e um painel Gio ao lado. Dentro do viewport, um cubo texturizado gira, desenhado com GLES 3.0 via ANGLE pelo renderizador customizado do Gio. Os painéis Gio são desenhados por cima no mesmo frame. O spike prova o caminho de renderização e fecha as 2 decisões que o resto do projeto herda. Sem cgo: as chamadas EGL e GLES saem por carregamento dinâmico de DLL.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] O módulo Go existe e o executável compila sem toolchain C, com `libEGL.dll` e `libGLESv2.dll` do ANGLE ao lado do executável
- [ ] O cubo texturizado gira no viewport, com um painel lateral e um botão Gio visíveis por cima
- [ ] Redimensionar a janela mantém o cubo proporcional e o painel no lugar
- [ ] Um clique no viewport chega ao código do viewport e um clique no botão chega ao Gio, sem um vazar para o outro
- [ ] ADR registrando a profundidade: Z reverso se o ANGLE expuser `GL_EXT_clip_control`, senão a alternativa escolhida e o motivo
- [ ] ADR registrando as texturas: DXT enviado nativo (`GL_EXT_texture_compression_s3tc`) ou decodificado para RGBA8 na CPU, e o motivo
