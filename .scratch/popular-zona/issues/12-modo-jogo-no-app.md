# 12: Modo jogo no app

**What to build:** um botão flutuante "Jogar" no viewport dos 2 modos coloca o humano embutido no chão sob o centro da tela e deixa o usuário andar (WASD), correr (Shift), pular (Espaço), voar sem colisão (F), alternar primeira e terceira pessoa (V) e olhar arrastando o mouse, como o voo de hoje (spec D8). A colisão usa terreno, BSP sólido (sem a flag 0x8) e as meshes que bloqueiam: `internal/unreal` passa a ler `bCollideActors`, `bBlockActors`, `bBlockPlayers`, `bWorldGeometry` e `bBlockNonZeroExtentTraces` com o padrão da classe quando a propriedade não vem gravada, como o `play_collision` do UE2-Studio. A BVH é preparada fora do laço de eventos com o aviso de carga. O humano anima idle, corrida, queda e pulo pelo estado da cápsula, com transição de 0,2 s e yaw pelo movimento, desenhado pelo passe de modelos. No modo população os monstros da prévia ficam visíveis e sem colisão; no modo de zonas o contorno das zonas continua desenhado. Esc sai e restaura a câmera de edição; o estado do editor do modo ativo (seleção, histórico) não é tocado. Textos do HUD no catálogo `spawn`, nos 2 idiomas.

**Blocked by:** 01, 10, 11

**Status:** resolved

- [x] Smoke no app (população): na clareira, o personagem anda entre os monstros, é barrado por cercas, pedra e troncos, sobe rampa de terreno até o limite e pula; F voa e V alterna a câmera (screenshots)
- [x] Smoke no app: a câmera em terceira pessoa encurta perto de uma parede
- [x] Smoke no app: o humano troca de animação entre parado, correndo, caindo e pulando
- [x] Smoke no app (zonas): entrar e sair do modo jogo dentro de uma zona mantém a seleção e o histórico de desfazer, e Esc volta a câmera para onde estava
- [x] Teste com cliente real (pulado sem `ZB_CLIENT`): as flags de colisão lidas de um tile batem com o padrão da classe quando a propriedade não vem gravada (pelo menos uma mesh que bloqueia e uma que não bloqueia)
- [x] Conferência da proporção humano × monstro registrada nas notas

## Comments

Integrado em `popular-zona`, a partir de `pz/12-modo-jogo-no-app` (`544cf1e`). Smoke de controles, colisão do cenário, animações, câmera, monstros sem colisão e proporção humano 80 × monstro 66,23 registrados nas notas. A proporção contra o cliente real do jogo ainda depende do ticket 13.

Revisão: colisão independente da visibilidade inclui meshes bloqueantes ocultas e BSP sólido invisível. Regressão serializada da cena e cápsula real passou. Mudanças no conjunto de tiles pausam/reconstroem a BVH mantendo a sessão; substituir o World encerra o jogo e invalida resultados antigos. O smoke registrou reconstrução para 6, 7, 8 e 9 tiles, retomada no chão e voo, saída com a câmera restaurada e troca para 18_16 sem manter a sessão antiga.

Os controles do editor ficam bloqueados durante preparação/jogo. Smoke final nos 2 modos: tentativas de alterar quantidade/nome/altura e comandos não modificaram campos nem documentos; após Esc, projetos byte a byte iguais ao baseline. No modo zonas, Undo desfez a subida de 64 feita antes de jogar, não uma entrada pendente. Corrigida também a drenagem Gio que antes mudava buffers de texto apesar de não aplicar comandos.

Build, vet e suíte completa com `ZB_CLIENT` passaram no gate final. Evidências: `C:/Workspace/zone-builder-notes/popular-zona/12-modo-jogo-no-app.md`, `12-evidence/`, `review-fixes.md` e `review-evidence/`.
