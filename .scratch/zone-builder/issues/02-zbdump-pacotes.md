# 02: Abrir pacotes e listar exports (`zbdump`)

**What to build:** uma CLI de diagnóstico que abre qualquer pacote do cliente L2 (`.unr`, `.utx`, `.usx`) e lista nomes, imports e exports. Ela entende pacotes crus, `Lineage2Ver111` (XOR 0xAC) e `Lineage2Ver121` (XOR com o byte baixo da soma do nome do arquivo em minúsculas, com recuperação da chave pelo magic). Recebendo a pasta do cliente, varre todos os pacotes e relata cada falha com o nome do pacote e o motivo. É o port da parte de leitura do `package-engine` do UE2-Studio: header, tabelas e compact index.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] `zbdump <pacote>` lista nomes, imports e exports com classe, pacote pai e tamanho serializado
- [x] Um pacote `Lineage2Ver121` renomeado abre pela recuperação da chave
- [x] Um container não suportado (120, 211/212, 411-414) gera o erro "versão de container não suportada" com o número da versão
- [x] `zbdump` com a pasta do cliente Majestic World varre todos os `.unr`, `.utx` e `.usx` sem nenhum erro de leitura
- [x] Em 1 mapa, 1 `.utx` e 1 `.usx` de amostra, as contagens de exports batem com o UE2-Studio
- [x] Testes do seam Cena contra o cliente real (varredura e Ver121 renomeado), apontados por variável de ambiente e pulados quando ela não existe

## Comments

Resolvido na branch de integração `zone-builder`. Branch `zb/02-zbdump-pacotes`. Varredura do cliente Fafurion (alvo por decisão do usuário; o Majestic World ficou indisponível): 1371 pacotes, 0 falhas. Contagens de nomes/imports/exports iguais ao UE2-Studio em todos os 1371 pacotes. Testes em `internal/l2pkg/client_test.go` (pulados sem `ZB_CLIENT`).
