# 01: Tela inicial e modos

**What to build:** o app abre numa tela inicial com 2 cartões, "Construir zonas" e "Popular zona", cada um com ícone Lucide, título e uma frase (spec D1). O estado do editor sai das variáveis locais do laço de eventos para um tipo de modo com 3 valores (início, zonas, população); cada modo é dono do seu editor e do seu painel no inspetor, e a seção do mapa, o viewport, a câmera, a barra de comandos e o projeto ficam compartilhados. "Construir zonas" leva ao editor de hoje sem nenhuma diferença; "Popular zona" abre o viewport com a seção do mapa e o painel de áreas vazio. Um botão "Início" na barra de comandos volta à tela inicial. Atalhos, eventos do viewport, desfazer e refazer vão só para o modo ativo. A flag `-mode zones|populate` pula a tela inicial; sem ela o app sempre abre na tela inicial, mesmo com `-project`. Os textos novos entram num catálogo `spawn` de `internal/locale`, em pt-BR e en, coberto pelos testes de catálogo (spec D9).

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] O app abre na tela inicial, com os 2 cartões e a frase de cada um, em pt-BR e en
- [ ] "Construir zonas" leva ao editor de zonas, que funciona como antes (criar, editar vértices, desfazer, compilar)
- [ ] "Popular zona" mostra o viewport, a seção do mapa e o painel de áreas vazio
- [ ] Ir e voltar pelo botão "Início" mantém os tiles abertos, a câmera e o projeto
- [ ] Atalhos e desfazer/refazer só agem no modo ativo
- [ ] `-mode zones` e `-mode populate` abrem direto no modo; sem a flag, a tela inicial aparece mesmo com `-project`
- [ ] O catálogo `spawn` existe nos 2 idiomas e passa nos testes de catálogo de `locale`
