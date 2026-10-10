# Ícone do Zone Builder

- `zonebuilder.svg`: fonte do ícone (256×256): um prisma de zona translúcido sobre um plano de chão isométrico, com alças de vértice e um pino marcando o ponto clicado. Fundo em gradiente grafite com acento ciano (face direita do prisma e centro do pino); a borda clara de 35% de opacidade separa o ícone do tema escuro (#202020) do Windows 11.
- `zonebuilder.ico`: gerado do SVG, com PNGs de 16, 20, 24, 32, 40, 48, 64, 128 e 256 px.
- `cmd/zonebuilder/rsrc_windows_amd64.syso`: o `.ico` como recurso #1 do executável, que o Gio usa na janela e na barra de tarefas.
- `icon.go`: embute o SVG (pacote `appicon`); a interface desenha o ícone como marca no canto superior esquerdo, pelo renderizador de SVG de `internal/ui/icon`.

Para regenerar depois de mudar o SVG:

```sh
cd assets/icon
npm i --no-save @resvg/resvg-js
node build-ico.mjs zonebuilder.svg zonebuilder.ico
cd ../../cmd/zonebuilder
go generate
```
