# 01: Núcleo de localização e cobertura

**What to build:** módulo único `internal/locale` com pt-BR padrão e en, catálogos embutidos por área, chaves estáveis, formatação de argumentos nomeados, plural (1 singular; 0 e 2 plural), números e percentuais de apresentação. Não mudar entrada numérica, documento ou XML. Verificador automatizado descobre pares de catálogos e usos no código; falha para chave ausente/duplicada, parâmetros diferentes, plural incompleto e uso sem tradução. Fornecer API pequena para os outros tickets, documentá-la no comentário do ticket antes de concluir.

Blocked by: None

Status: resolved

- [x] Em ambos os idiomas, mensagens estáticas/dinâmicas e plural 0/1/2 resolvem com números próprios; chave inválida não cai silenciosamente para português.
- [x] Verificador detecta área nova sem par ou com chave/parâmetros/variantes divergentes e chaves usadas pelo código sem catálogo.
- [x] Catálogos embutidos, sem rede nem recarga por quadro; instruções de extensão em documentação de desenvolvimento.

## Comments

- API: `locale.Language`, `PtBR`, `En`, `Normalize(string) Language`; `Text(lang, key) string` para mensagens estáticas; `Format(lang, key, map[string]string) string` para argumentos nomeados pré-formatados; `Plural(lang, key, count, map[string]string) string` com `{count}` localizado; `Number(lang, value float64, decimals int) string` e `Percent(lang, fraction float64, decimals int) string` somente para apresentação; `Message{Key, Args, Parts, Count, Plural}.Render(lang)` reapresenta status e partes traduzidas sem repetir a ação.
- Catálogos: `internal/locale/catalog/<área>/{pt-BR,en}.json`, com objeto JSON chave→texto ou chave→`{"one":"...","other":"..."}`; novas áreas com ambos arquivos são descobertas automaticamente. `locale.Verify(fs.FS)` confere catálogos embutidos e usos no código Go; `locale.Check(catalogFS, sourceFS)` permite conferir entradas isoladas. Chaves dinâmicas falham na verificação; usar chaves literais em chamadas e mensagens, inclusive nas partes de status retidos. Instruções de extensão em `README.md`.
- Evidência escrita: `locale_test.go` cobre idioma padrão/inválido, texto estático, argumentos, plural 0/1/2, números negativos/percentuais, chave desconhecida e reapresentação de status; `check_test.go` cobre par válido, idioma ausente, chave faltante/duplicada, parâmetros incompatíveis, variantes plurais e chave usada sem tradução. Testes não executados neste ramo; integração os executará.
