# Zone Builder

Editor desktop de zonas para servidores de Lineage II. O Zone Builder abre os mapas do cliente em 3D, deixa marcar as zonas com o mouse direto sobre o terreno e gera o XML no formato que o servidor Java carrega de `data/zone/`.

## O que ele faz

- Abre os tiles do cliente (`Maps/X_Y.unr`) com terreno, BSP e static meshes texturizados, sozinhos ou com os vizinhos (até 3×3).
- Cria zonas por polígono, retângulo, círculo ou o tile inteiro, inclusive exclusões (`banned_polygon`), e adiciona restart points e PK restart points.
- Cada clique cai no ponto visível sob o cursor, em coordenadas do servidor.
- Edita vértices, shapes, faixa Z, altura da zona, tipo e parâmetros `<set>`, com desfazer e refazer.
- Mostra os problemas de cada zona enquanto você edita, como polígono que se cruza, nome repetido ou faixa Z invertida, e bloqueia a compilação até corrigir.
- Compila as zonas selecionadas em um arquivo por tipo (`zonebuilder_<tipo>.xml`) e mostra o XML numa janela com botão de copiar.
- Guarda o trabalho em projetos `.zbproj`, inclusive zonas ainda incompletas.

## Requisitos

- Windows 64 bits.
- Go 1.27 ou mais novo, para compilar.
- PowerShell 7 (`pwsh`) e `make`.
- Um cliente de Lineage II com a pasta `Maps`.

## Compilar e executar

```sh
make build   # gera "bin/Zone Builder.exe" com as DLLs do ANGLE ao lado
make run     # compila e abre o app
make run ARGS="-project giran.zbproj"
```

A versão exibida no título da janela vem de `APP_VERSION`, no arquivo `.env`.

Opções de linha de comando:

| Opção | Uso |
|---|---|
| `-client` | pasta do cliente (a que contém `Maps`) |
| `-tile` | tile que o campo traz preenchido, por exemplo `22_22` |
| `-project` | projeto `.zbproj` aberto ao iniciar |
| `-camera` | pose inicial da câmera, `x,y,z,yaw,pitch` |
| `-fps` | mede a taxa de quadros e registra no log |

Sem `-client`, o app usa a última pasta salva na configuração do usuário (`%AppData%\ZoneBuilder\config.json`), depois a variável de ambiente `ZB_CLIENT`.

## Como usar

1. Informe a pasta do cliente e o tile, e clique em **Abrir**.
2. Em **Nova zona**, digite o nome, escolha o tipo e clique em **Criar zona e desenhar**.
3. Clique sobre o mapa para marcar os vértices; Enter ou um clique no primeiro vértice fecha o polígono.
4. Ajuste altura, tipo e parâmetros no inspetor à direita e na janela **Altura da zona**.
5. Clique em **Compilar XML**, copie cada arquivo da janela e cole em `data/zone/` do servidor com o nome indicado.

Controles do viewport:

| Tecla ou gesto | Ação |
|---|---|
| W, A, S, D | andar e mover para os lados |
| E, Q | subir e descer |
| Shift | velocidade alta |
| arrastar | olhar em volta |
| roda do mouse | avançar e recuar |
| PageUp, PageDown | subir e descer a zona selecionada |
| Enter | fechar o polígono em desenho |
| Delete | apagar o vértice selecionado |
| Esc | soltar a ferramenta |
| Ctrl+Z, Ctrl+Y | desfazer e refazer |

## Licenças de terceiros

- ANGLE (`third_party/angle`): BSD.
- Ícones Lucide (`internal/ui/icon/lucide`): ISC.
- Fonte Inter (`internal/ui/fonts`): SIL Open Font License.
