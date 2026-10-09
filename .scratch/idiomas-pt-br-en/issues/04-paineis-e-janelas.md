# 04: Interface dos painéis e janelas

**What to build:** localizar todos os textos de apresentação em `internal/ui` (barra de comandos, mapa, zonas, propriedades, edição, lista de problemas, dock, viewport, menus, janela de altura e XML, cartões e dicas), inclusive títulos, botões, estados vazios, placeholders e instruções. Usar idioma do `Shell` do ticket 02; não recriar widgets ao mudar. Colaborar com ticket 03 para modelo de linhas. Ajustar larguras/layout para inglês em 1280×800; campos, clique, posição, scroll e janelas mantidos. XML e nomes/tipos do servidor não se traduzem.

Blocked by: 01, 02

Status: resolved

- [x] Todos os rótulos e dicas expostos pelo UI refletem o idioma atual na próxima apresentação.
- [x] Valores de usuário, tokens do servidor, campos e XML ficam literais.
- [x] Troca com editor focado e janelas abertas preserva foco e layout; texto inglês não cobre outras áreas.

## Comments

- 2026-10-09: Assumido no ramo `i18n/04-ui`, baseado em `c918e4b`.
- 2026-10-09: Catálogos `ui` nos dois idiomas, textos consultados durante cada apresentação; estado dos widgets e XML preservado, cabeçalhos/contagens localizados, descrição do filtro nativo por idioma e largura da barra limitada para caber ao lado do inspetor. Testes de apresentação sem GPU adicionados; execução e smoke visual em 1280×800 ficam para a integração.
