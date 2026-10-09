# Ícone do Zone Builder

- `zonebuilder.svg`: fonte do ícone (256×256): um prisma de zona translúcido sobre um plano de chão isométrico, com alças de vértice e um pino marcando o ponto clicado. O fundo turquesa claro garante contraste nos temas escuro (#202020) e claro (#F3F3F3) do Windows 11.
- `zonebuilder.ico`: gerado do SVG, com PNGs de 16, 20, 24, 32, 40, 48, 64, 128 e 256 px.
- `cmd/zonebuilder/rsrc_windows_amd64.syso`: o `.ico` como recurso #1 do executável, que o Gio usa na janela e na barra de tarefas.

Para regenerar depois de mudar o SVG:

```sh
cd assets/icon
npm i --no-save @resvg/resvg-js
node build-ico.mjs zonebuilder.svg zonebuilder.ico
cd ../../cmd/zonebuilder
go generate
```
