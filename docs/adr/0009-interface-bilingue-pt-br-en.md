# Interface bilíngue pt-BR e en

A interface do app pode ser usada em português (pt-BR) ou em inglês (en). A mecânica dos catálogos (arquivos, chaves literais, `locale.Message`, regra `one`/`other`) está em [Traduções para desenvolvimento](../desenvolvimento.md); este ADR registra por que o sistema é assim.

## Decisão

- **2 idiomas, sem detecção.** Os idiomas são exatamente `pt-BR` (padrão) e `en`. Preferência ausente ou inválida vira pt-BR em `locale.Normalize`. O app não lê o idioma de exibição do Windows. As opções do seletor sempre dizem "PT-BR" e "EN", qualquer que seja o idioma ativo.
- **Preferência pessoal, fora do projeto.** O idioma fica só em `project.Config`. O formato e a versão do `.zbproj` não mudam, e abrir um projeto nunca troca a preferência. A escolha é salva assim que muda, e trocar o idioma nunca marca o projeto como não salvo.
- **Catálogos embutidos.** Os catálogos são compilados no binário: sem arquivos de tradução externos em tempo de execução, sem rede e sem recarga por frame.
- **Trocar só redesenha.** A troca nunca recria a janela, o Shell, os widgets de edição, o documento ou a cena 3D, e nunca roda de novo um comando, a validação, o carregamento do mapa ou a medição do perfil do chão.
- **Texto guardado é mensagem, não string.** Todo texto produzido por uma ação e mantido na tela (status, instrução da ferramenta, linhas de problema, textos de cobertura) guarda a identidade da mensagem e seus dados (`locale.Message{Key, Args, Parts, Count, Plural}`), nunca a string já traduzida. É assim que o texto já na tela muda de idioma.
- **Validação independente do idioma.** A validação das zonas guarda regras e valores brutos, e a apresentação é montada a partir deles. Refazer a lista de problemas só porque o idioma mudou mantém a ordem, a contagem e o índice usado pelo clique (zona, shape, vértice, restart point).
- **Erros com contexto e detalhe original.** Um erro nunca é traduzido comparando o texto em português de `Error()` ou do log. A interface junta a descrição traduzida da operação ao detalhe original do sistema, por exemplo `actions.preference.unsaved` mais o erro bruto.
- **Corte completo.** As strings em português substituídas saíram dos caminhos de apresentação; não há um sistema de mensagens paralelo.
- **Frases completas.** Fragmentos traduzidos nunca são concatenados: cada variante é uma frase inteira com valores interpolados. Contagem 1 escolhe `one`; 0 e as demais escolhem `other`.
- **Só a apresentação segue o idioma.** Medidas exibidas, porcentagens, rótulos e frases seguem as convenções do idioma. Nomes dados pelo usuário, ids de tile, coordenadas, valores de parâmetros, tokens `type` do servidor, XML, arquivos de projeto e números digitados nunca são localizados.

## Fora do escopo

- Outros idiomas e a detecção automática do idioma do sistema.
- Traduções editáveis pelo usuário e pacotes de idioma baixáveis.
- A interface nativa do Windows: nos diálogos de arquivo e pasta, o app traduz só o título e a descrição ou o filtro que passa, no idioma ativo ao abrir o diálogo; um diálogo já aberto não muda de idioma.
- Logs técnicos, comentários de código, identificadores, valores de protocolo, os docs do repositório e as ferramentas de diagnóstico (como o `zbdump`), que ficam em português.

## Consequences

- Uma mensagem nova precisa de entrada nos 2 catálogos; `go test ./internal/locale` confere.
- Em inglês, chão é "ground" e piso é "floor"; os termos estão em [`GLOSSARY.md`](../../GLOSSARY.md). Traduzir os textos da água não muda o deslocamento nem a regra de criação da zona de água ([ADR 0005](0005-agua-30-abaixo-do-volume.md)).
- Se a preferência não puder ser salva, o idioma escolhido vale na sessão, e o status avisa que ele não será lembrado.
