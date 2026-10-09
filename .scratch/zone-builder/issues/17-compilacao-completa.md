# 17: Compilação completa e aceitação no servidor

**What to build:** o usuário escolhe quais zonas compilar e a pasta de saída e recebe um arquivo por tipo de zona, com prefixo próprio do Zone Builder, para não sobrescrever arquivos de zona que já existem lá. Zonas saem em ordem de nome. Ao fim, o app mostra onde cada arquivo foi gravado. A aceitação final é carregar o resultado no servidor Java sem nenhuma alteração nele.

**Blocked by:** 16 (Validação e painel de problemas)

**Status:** resolved

- [x] Só as zonas selecionadas entram na compilação
- [x] Sai 1 arquivo por tipo presente na seleção, com o prefixo próprio, e nenhum arquivo existente com outro nome é tocado
- [x] Ao fim da compilação, o app lista o caminho de cada arquivo gravado
- [x] Um projeto com 1 zona de cada um dos 23 tipos compila, e o servidor Java sem alteração carrega tudo sem `invalid territory data`, `Empty territory` ou exceção

## Comments

Resolvido na branch de integração `zone-builder`. Branch `zb/17-compilacao-completa`. Seleção por zona, 1 arquivo por tipo com prefixo `zonebuilder_`, arquivos sem o prefixo intocados, caminhos listados ao fim. Projeto com 1 zona de cada um dos 23 tipos: `LOADED 23`, exit 0, sem `invalid territory data`, `Empty territory` ou exceção no harness com a SkillTable real. Depois da revisão, recompilar também remove os `zonebuilder_*.xml` antigos. Manual: carregar no GameServer real.

Em 2026-10-08, por pedido do usuário, a compilação deixou de gravar arquivos: o XML de cada tipo aparece numa janela flutuante com o nome sugerido (`zonebuilder_<tipo>.xml`) e um botão Copiar. Saíram a pasta de saída (painel, flag `-out`, projeto e configuração) e a remoção de arquivos antigos.
