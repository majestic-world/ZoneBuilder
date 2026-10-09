# 01: Núcleo de localização e cobertura

**What to build:** módulo único `internal/locale` com pt-BR padrão e en, catálogos embutidos por área, chaves estáveis, formatação de argumentos nomeados, plural (1 singular; 0 e 2 plural), números e percentuais de apresentação. Não mudar entrada numérica, documento ou XML. Verificador automatizado descobre pares de catálogos e usos no código; falha para chave ausente/duplicada, parâmetros diferentes, plural incompleto e uso sem tradução. Fornecer API pequena para os outros tickets, documentá-la no comentário do ticket antes de concluir.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] Em ambos os idiomas, mensagens estáticas/dinâmicas e plural 0/1/2 resolvem com números próprios; chave inválida não cai silenciosamente para português.
- [ ] Verificador detecta área nova sem par ou com chave/parâmetros/variantes divergentes e chaves usadas pelo código sem catálogo.
- [ ] Catálogos embutidos, sem rede nem recarga por quadro; instruções de extensão em documentação de desenvolvimento.

## Comments
