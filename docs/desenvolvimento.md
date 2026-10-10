# Desenvolvimento

[Apresentação do projeto](../README.md) · [Guia de uso](uso.md)

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
make dist    # compila e gera "dist/Zone Builder By Mk v<versão>.zip" com o conteúdo de bin/, apagando os zips antigos de dist/
```

A versão exibida no título da janela vem de `APP_VERSION`, no arquivo `.env`.

O executável é um app de interface do Windows (`-H windowsgui`): aberto pelo Explorer ou por atalho, não abre janela de terminal. Iniciado de um terminal (inclusive por `make run`), o log aparece nesse terminal.

Opções de linha de comando:

| Opção | Uso |
|---|---|
| `-client` | pasta do cliente (a que contém `Maps`) |
| `-tile` | tile que o campo traz preenchido, por exemplo `22_22` |
| `-project` | projeto `.zbproj` aberto ao iniciar |
| `-camera` | pose inicial da câmera, `x,y,z,yaw,pitch` |
| `-fps` | mede a taxa de quadros e registra no log |
| `-mode` | abre direto num modo, sem a tela inicial: `zones` (Construir zonas) ou `populate` (Popular zona); sem a opção, o app abre na tela inicial, mesmo com `-project` |

Sem `-client`, o app usa a última pasta salva na configuração do usuário (`%AppData%\ZoneBuilder\config.json`), depois a variável de ambiente `ZB_CLIENT`.

## Traduções para desenvolvimento

Mensagens da interface ficam embutidas em `internal/locale/catalog/<área>/pt-BR.json` e `en.json`. Ao acrescentar uma área, crie **os dois arquivos**: cada chave estável começa com `<área>.`, e os dois idiomas precisam conter as mesmas chaves, os mesmos parâmetros nomeados (`{name}`) e, para mensagens com contagem, as variantes `one` e `other`. A forma `one` vale somente para 1; `other` vale para 0 e demais contagens. Use uma frase completa por variante, não fragmentos concatenados.

Apresente texto estático com `locale.Text`, texto parametrizado com `locale.Format` e contagem com `locale.Plural`. Números exibidos usam `locale.Number` ou `locale.Percent` antes da interpolação; coordenadas, valores digitados, projetos e XML permanecem literais. Para status que sobrevivem à troca de idioma, retenha `locale.Message` com chave e dados; submensagens em `Parts` são formatadas no idioma corrente ao chamar `Render`. **Todas as chaves usadas em chamadas e mensagens precisam ser literais**, inclusive nos mapas de `Parts`: `go test ./internal/locale` rejeita chaves dinâmicas e confere automaticamente pares, parâmetros, variantes, duplicatas e usos sem tradução, sem lista manual de áreas.

## Licenças de terceiros

- ANGLE (`third_party/angle`): BSD.
- Ícones Lucide (`internal/ui/icon/lucide`): ISC.
- Fonte Inter (`internal/ui/fonts`): SIL Open Font License.
