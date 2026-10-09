Status: resolved

# Idiomas da interface: português brasileiro e inglês

## Problem Statement

O Zone Builder só apresenta a interface em português brasileiro. Quem prefere trabalhar em inglês não consegue mudar o idioma, e textos aparecem em muitos pontos distintos: controles, dicas, mensagens de ações, avisos de chão, problemas de zona, diálogos e números. Traduzir apenas os painéis deixaria a experiência misturada. Um módulo novo também poderia acrescentar textos só em um idioma sem que ninguém percebesse.

O usuário precisa escolher entre pt-BR (padrão) e inglês, trocar rapidamente durante o trabalho e reencontrar sua escolha ao reabrir o app, sem modificar o projeto nem comprometer as regras de edição, validação e compilação de XML.

## Solution

Exibir um controle compacto e acessível **PT-BR / EN** na parte superior da janela. Ao escolher um idioma, todos os textos do app voltados ao usuário passam a ser exibidos naquele idioma na próxima apresentação, inclusive mensagens e avisos já visíveis. A escolha fica nas preferências do usuário; sem preferência válida o app usa pt-BR. O documento, a seleção, os campos em edição e o XML mantêm seus valores.

Um único módulo de localização concentra os catálogos embutidos, a escolha do idioma e a formatação da apresentação. Cada área funcional fornece o mesmo conjunto de mensagens em pt-BR e inglês, com verificação automatizada de cobertura para impedir que módulos futuros introduzam textos sem a segunda tradução.

## User Stories

1. Como pessoa que já usa o Zone Builder, quero que a primeira inicialização continue em pt-BR, para não mudar minha experiência sem consentimento.
2. Como pessoa que usa o app em inglês, quero selecionar EN num controle sempre acessível no topo da janela, para trabalhar sem procurar uma opção no inspetor.
3. Como pessoa que usa o app em português, quero voltar a PT-BR com um clique, para alternar sem reiniciar.
4. Como pessoa que alterna idiomas, quero identificar qual opção está ativa, para não confundir o idioma atual com o idioma que o botão vai ativar.
5. Como pessoa que reabre o app, quero que a última escolha de idioma seja restaurada, para não refazê-la em cada sessão.
6. Como pessoa que usa uma configuração antiga, quero que a ausência da preferência mantenha pt-BR, para continuar abrindo o app normalmente.
7. Como pessoa cuja configuração contém um idioma não reconhecido, quero que o app continue em pt-BR, para não ficar sem interface utilizável.
8. Como pessoa que abre projetos diferentes, quero que o idioma continue sendo minha preferência pessoal, para não mudar quando troco de projeto.
9. Como pessoa que está digitando nome, parâmetro, coordenadas ou faixa Z, quero alternar idiomas sem perder o conteúdo nem o foco do campo, para continuar a edição.
10. Como pessoa que está editando uma zona, quero que seleção, ferramenta ativa, zona oculta, escolha de compilação e histórico de desfazer/refazer sobrevivam à troca, para não repetir trabalho.
11. Como pessoa com o inspetor rolado e janelas flutuantes abertas, quero que a troca preserve posições, rolagem e janelas, para não perder meu contexto visual.
12. Como pessoa que alterna idiomas, quero que a mudança de preferência não marque o projeto como não salvo, para não confundir configuração do app com alteração da zona.
13. Como pessoa que está carregando um mapa, quero que mensagens de progresso e avisos apareçam no idioma escolhido sem reiniciar a carga, para acompanhar o trabalho em andamento.
14. Como pessoa que usa a barra de comandos, quero nomes e dicas de controles no idioma selecionado, para saber o que cada ação faz.
15. Como pessoa que usa os painéis de mapa, zonas, propriedades e edição, quero títulos, botões, rótulos, estados vazios e sugestões no idioma escolhido, para entender a interface inteira.
16. Como pessoa que usa o viewport, quero que ferramentas, comandos flutuantes, status do cursor e instruções da ferramenta ativa reflitam o idioma escolhido, para agir com segurança.
17. Como pessoa que recebe o resultado de salvar, abrir, editar, copiar ou compilar, quero ler o resultado no idioma ativo, inclusive após alternar com a mensagem ainda visível, para entender o que aconteceu.
18. Como pessoa que recebe um problema de validação, quero ler sua causa e os valores relevantes no idioma atual, para poder corrigir a zona.
19. Como pessoa que alterna idiomas com a lista de problemas aberta, quero que as mensagens mudem sem mudar a ordem, o alvo do clique nem o número de problemas, para continuar navegando até o mesmo erro.
20. Como pessoa que recebe um aviso de cobertura, quero sua explicação em inglês ou pt-BR sem que isso o transforme em erro bloqueante, para distinguir risco de impedimento de compilação.
21. Como pessoa que usa o inspetor e a janela de altura, quero ver chão, outras camadas, folga do piso, folga do topo, cobertura, áreas e pior ponto no idioma atual, para interpretar as medidas corretamente.
22. Como pessoa que lê medidas, quero separadores de milhar, decimais, percentuais e singular/plural adequados ao idioma, para não interpretar números incorretamente.
23. Como pessoa que usa a janela XML, quero seus títulos e ações no idioma atual, mas o XML copiado byte a byte igual ao compilado antes da troca, para manter a compatibilidade com o servidor.
24. Como pessoa que abre diálogos nativos de arquivo e pasta, quero seus títulos e descrições fornecidos pelo app no idioma ativo ao abri-los, para reconhecer a operação.
25. Como pessoa que digitou nomes de zona, nomes de parâmetros e caminhos, quero que esses dados permaneçam literais quando mudar o idioma, para não corromper o projeto ou a exportação.
26. Como pessoa que usa os tipos de zona, quero que os valores exigidos pelo servidor continuem idênticos no documento e no XML, para que a tradução visual não altere o protocolo.
27. Como pessoa que perdeu acesso à gravação de preferências, quero ser informada no idioma ativo de que a mudança desta sessão não será lembrada, para não presumir persistência inexistente.
28. Como pessoa que usa a janela padrão, quero que as opções e textos ingleses continuem legíveis sem cobrir o viewport nem os outros controles, para não perder espaço de trabalho.
29. Como pessoa que desenvolve um módulo novo, quero acrescentar as mensagens dos dois idiomas no mesmo fluxo de trabalho e ter uma verificação que detecte lacunas, para não publicar interface mista.
30. Como pessoa que mantém as traduções, quero detectar parâmetros e formas plurais incompatíveis entre os catálogos, para não descobrir mensagens quebradas somente durante o uso.
31. Como pessoa que usa o app, quero que mensagens técnicas que cheguem à interface tenham contexto traduzido sem esconder o detalhe útil do erro original, para entender falhas recuperáveis.
32. Como pessoa que usa o app, quero alternar durante o desenho de um polígono ou durante a medição do perfil do chão sem alterar a geometria nem recomeçar o cálculo, para manter a continuidade da tarefa.

## Implementation Decisions

1. **Idiomas suportados e padrão.** Aceitar `pt-BR` e `en`. Preferência ausente ou inválida resolve para `pt-BR`; não detectar automaticamente o idioma do Windows nesta entrega. Exibir os nomes das opções de forma estável como PT-BR e EN, independentemente do idioma ativo.
2. **Propriedade da preferência.** Acrescentar idioma somente à configuração do usuário. O formato e a versão do projeto não mudam; abrir um projeto nunca sobrescreve a preferência. Salvar a escolha assim que ela mudar. Se a gravação falhar, preservar a escolha na sessão e apresentar aviso de que não foi persistida.
3. **Seam de localização.** Criar um único módulo de localização com interface pequena para consultar uma mensagem pelo idioma e pela chave e para formatar argumentos, contagens e números apresentados. Os catálogos fazem parte do binário; não há arquivos de tradução externos obrigatórios em runtime, dependência de rede ou reload por frame. As mensagens estáticas não exigem formatação dinâmica.
4. **Organização para módulos futuros.** Chaves estáveis e delimitadas por área funcional; cada área nova inclui o par de catálogos pt-BR/en. A verificação descobre os pares e compara também as chaves efetivamente usadas pelo código, sem uma lista manual de módulos a atualizar. Adicionar uma área significa incluir suas duas traduções, chamá-las na apresentação e executar a verificação. IDs de mensagens não dependem da redação atual.
5. **Contrato dos catálogos.** As duas versões de cada chave têm os mesmos argumentos nomeados e variantes de plural necessárias. Não concatenar fragmentos de frases traduzidas quando a gramática difere; uma mensagem completa recebe os valores interpolados. Para contagem, `1` seleciona o singular e `0` e demais quantidades selecionam o plural nos idiomas desta entrega. Chaves ausentes, duplicadas, parâmetros incompatíveis e plural incompleto falham nas verificações, em vez de aparecerem como ID ou português silenciosamente em inglês.
6. **Troca em tempo de execução.** O loop da janela é dono do idioma selecionado. O controle de troca fica na composição superior, fora do inspetor rolável, sem entrar nos eventos de clique do viewport. Na troca, os controles são reapresentados com o novo idioma; não recriar a janela, o Shell, os widgets de edição, o documento nem a cena 3D.
7. **Estado textual transitório.** Texto produzido por uma ação e mantido na tela deve guardar identidade da mensagem e seus dados, não somente uma frase já traduzida. O mesmo vale para instruções e status derivados do estado atual. Uma troca reformatará os itens ainda visíveis; ela não repete comandos, validação, carga de mapa nem medição.
8. **Problemas e avisos.** A validação conserva regras e dados estruturados independentes de idioma; texto de apresentação nasce das regras, variantes e valores. O mapeamento de erro para zona, shape, vértice ou ponto de restart e o comportamento de bloqueio de compilação não mudam. Avisos de cobertura continuam separados dos problemas da zona e **não bloqueiam** a compilação, conforme a decisão arquitetural sobre chão medido. A apresentação da lista deve ser refeita também quando só o idioma mudou, preservando o índice que o clique utiliza.
9. **Mensagens externas ao painel.** Migrar mensagens exibidas de ações, seleção de água, cobertura, carga e erros recuperáveis em todas as áreas do app. Uma camada de apresentação pode combinar uma descrição traduzida da operação com o detalhe original de um erro do sistema. Não usar a redação em português de `Error()` nem a de logs como contrato de tradução, nem traduzir erros por correspondência de texto.
10. **Dados versus apresentação.** Preservar literalmente nomes fornecidos pelo usuário, IDs de tile, coordenadas e valores de parâmetros; manter os valores exatos dos tipos de zona do servidor. XML compilado, arquivos de projeto e entradas numéricas continuam com o contrato de máquina existente. Somente medidas e percentuais apresentados, rótulos e frases usam convenções do idioma; não alterar a interpretação de números digitados.
11. **Diálogos nativos.** Traduzir títulos e descrições que o app envia ao Windows conforme o idioma no instante da abertura. Um diálogo já aberto não precisa mudar de idioma em tempo real; a linguagem do restante do diálogo é controlada pelo Windows.
12. **Layout.** Preferir um seletor compacto na região superior, sempre visível. Medir as versões inglesas mais longas na janela padrão e manter rótulos legíveis; nenhuma tradução pode invadir o inspetor, o dock ou o viewport por depender de larguras fixadas para a frase portuguesa.
13. **Cutover completo.** Remover as frases portuguesas substituídas dos caminhos de apresentação, em vez de manter dois sistemas de mensagens em paralelo. Logs internos, comentários, identificadores do código e valores do protocolo podem continuar no idioma e formato existentes.

## Testing Decisions

- **Princípio:** testes permanentes devem observar comportamento de quem usa o app: idioma efetivo, texto apresentado, preservação do documento e XML, persistência e destino dos cliques. Não testar listas internas de widgets, quantidade de chamadas de busca, cópias de strings nem detalhes de implementação dos catálogos. A revisão visual complementa os testes; não os substitui.
- **Seam principal:** usar a interface de apresentação que a janela já expõe para renderizar controles, status, problemas, avisos e medidas com um idioma selecionado; dirigir a troca no loop da janela e verificar a saída observável. Caso a montagem atual impeça um teste determinístico sem GPU, extrair somente a apresentação de estado e mensagens em um seam de alto nível compartilhado pelo loop e pela UI; não criar um seam novo em cada painel.
- **Seams existentes:** exercitar a configuração por carregar/salvar, o documento por validação/compilação e a UI por solicitações e apresentação, como já ocorre nos testes de projeto e de zona. Verificar que o resultado de validar e compilar não depende do idioma e que o idioma não entra no projeto.
- **Catálogos:** testar por comportamento que toda chave usada por uma área resolve nos dois idiomas e que mensagens com argumentos e plural funcionam com `0`, `1` e `2`. Um verificador de desenvolvimento compara cobertura, argumentos e variantes; ele deve falhar se um módulo novo trouxer chave sem tradução correspondente.
- **Persistência:** cobrir configuração antiga sem idioma, escolha salva e reaberta, valor inválido, falha de gravação e separação entre preferência e projeto, com arquivos temporários isolados.
- **Troca dinâmica:** com um projeto carregado, texto em edição, seleção, ferramenta ativa, problemas e aviso de chão visíveis, alternar pt-BR → en → pt-BR e conferir textos, estado preservado, número e ordem de linhas, comportamento do clique e ausência de alteração não salva. Exercitar um status gerado antes da troca.
- **Formatação:** usar números que distingam separador de milhar, vírgula/ponto decimal e plural, incluindo folga negativa, cobertura, outras camadas e área sem chão. Entradas e saída XML devem continuar byte a byte estáveis.
- **Prior art:** testes de ida e volta de persistência do projeto, testes de problemas e compilação de zona e testes de fluxo de água existentes dão o padrão de dados reais e resultado observável. Evitar testes de redação de logs e valores internos ocasionais.
- **Smoke manual obrigatório:** iniciar sem preferência e conferir pt-BR; abrir um projeto com problema e aviso de cobertura; trocar para EN com mapa em andamento, janela de altura e XML abertos; verificar telas e interação na janela padrão; fechar e reabrir em EN; voltar para pt-BR. Conferir que o XML copiado não foi traduzido.

## Out of Scope

- Idiomas além de pt-BR e inglês; detecção automática do idioma do sistema.
- Tradução de nomes e parâmetros escolhidos pelo usuário, tokens do servidor, conteúdo do XML, coordenadas ou dados já salvos em projetos.
- Tradução da interface nativa do Windows, logs técnicos, comentários de código, documentação do repositório e comandos de diagnóstico para desenvolvedores.
- Edição de traduções pelo usuário, download de pacotes de idiomas e troca do idioma de diálogos nativos que já estejam abertos.
- Alterar a geometria do chão, o cálculo do perfil, critérios de aviso, política de bloqueio, formatos de projeto/XML ou o comportamento de compilação.

## Further Notes

- O glossário é a fonte dos termos portugueses de cobertura: **chão**, **camada**, **outras camadas**, **folga do piso**, **folga do topo**, **cobertura**, **sem chão** e **pior ponto**. Traduzir os conceitos para inglês sem reutilizar `floor` indistintamente para chão e piso; no app, piso é o limite `zmin`, chão é a superfície medida.
- O glossário também distingue **volume de água**, **topo** e **exata/aproximada**. A tradução da interface não muda o offset nem a regra de criação das zonas de água.
- O app já embute fontes e ícones e desenha cartões Gio sobre um viewport 3D; a troca de idioma só muda apresentação, não o renderer. A largura dos textos ingleses e a ordem de sobreposição dos cartões exigem prova visual.
- A especificação usa o seam de apresentação de mais alto nível e os seams de persistência/documento já disponíveis; não pressupõe uma interface por painel. Esse recorte é a proposta de verificação sem entrevista, conforme a solicitação de sintetizar a conversa existente.

## Comments

- Especificação derivada do plano de internacionalização discutido nesta conversa e do estado atual do app; pronta para decomposição em tickets.
