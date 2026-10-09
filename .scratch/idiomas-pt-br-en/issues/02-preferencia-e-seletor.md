# 02: Preferência e controle de idioma

**What to build:** guardar `pt-BR`/`en` em configuração do usuário; ausência/inválido -> pt-BR. O loop da janela controla o idioma, oferece seletor PT-BR / EN compacto, acessível e sempre visível no alto fora do inspetor, com opção ativa evidente. Persistir imediatamente, preservar idioma da sessão e mostrar aviso localizado se escrita falhar. Nunca alterar projeto, XML, foco/editor, seleção, ferramenta, janelas, rolagem nem disparar trabalho ao alternar. Facilitar apresentação reativa no `Shell` sem recriar widgets.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] Configuração antiga, escolha reaberta, inválido e falha de gravação cobertos com arquivos temporários.
- [ ] Troca pt-BR → en → pt-BR atualiza controle sem modificar estado de edição nem marcar documento como sujo.
- [ ] Seletor legível na janela 1280×800, sem interceptar cliques do viewport fora do cartão.

## Comments
