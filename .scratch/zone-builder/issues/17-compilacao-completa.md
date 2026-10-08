# 17: Compilação completa e aceitação no servidor

**What to build:** o usuário escolhe quais zonas compilar e a pasta de saída e recebe um arquivo por tipo de zona, com prefixo próprio do Zone Builder, para não sobrescrever arquivos de zona que já existem lá. Zonas saem em ordem de nome. Ao fim, o app mostra onde cada arquivo foi gravado. A aceitação final é carregar o resultado no servidor Java sem nenhuma alteração nele.

**Blocked by:** 16 (Validação e painel de problemas)

**Status:** ready-for-agent

- [ ] Só as zonas selecionadas entram na compilação
- [ ] Sai 1 arquivo por tipo presente na seleção, com o prefixo próprio, e nenhum arquivo existente com outro nome é tocado
- [ ] Ao fim da compilação, o app lista o caminho de cada arquivo gravado
- [ ] Um projeto com 1 zona de cada um dos 23 tipos compila, e o servidor Java sem alteração carrega tudo sem `invalid territory data`, `Empty territory` ou exceção
