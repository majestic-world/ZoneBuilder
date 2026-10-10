# 12: Modo jogo no app

**What to build:** um botão flutuante "Jogar" no viewport dos 2 modos coloca o humano embutido no chão sob o centro da tela e deixa o usuário andar (WASD), correr (Shift), pular (Espaço), voar sem colisão (F), alternar primeira e terceira pessoa (V) e olhar arrastando o mouse, como o voo de hoje (spec D8). A colisão usa terreno, BSP sólido (sem a flag 0x8) e as meshes que bloqueiam: `internal/unreal` passa a ler `bCollideActors`, `bBlockActors`, `bBlockPlayers`, `bWorldGeometry` e `bBlockNonZeroExtentTraces` com o padrão da classe quando a propriedade não vem gravada, como o `play_collision` do UE2-Studio. A BVH é preparada fora do laço de eventos com o aviso de carga. O humano anima idle, corrida, queda e pulo pelo estado da cápsula, com transição de 0,2 s e yaw pelo movimento, desenhado pelo passe de modelos. No modo população os monstros da prévia ficam visíveis e sem colisão; no modo de zonas o contorno das zonas continua desenhado. Esc sai e restaura a câmera de edição; o estado do editor do modo ativo (seleção, histórico) não é tocado. Textos do HUD no catálogo `spawn`, nos 2 idiomas.

**Blocked by:** 01, 10, 11

**Status:** ready-for-agent

- [ ] Smoke no app (população): na clareira, o personagem anda entre os monstros, é barrado por cercas, pedra e troncos, sobe rampa de terreno até o limite e pula; F voa e V alterna a câmera (screenshots)
- [ ] Smoke no app: a câmera em terceira pessoa encurta perto de uma parede
- [ ] Smoke no app: o humano troca de animação entre parado, correndo, caindo e pulando
- [ ] Smoke no app (zonas): entrar e sair do modo jogo dentro de uma zona mantém a seleção e o histórico de desfazer, e Esc volta a câmera para onde estava
- [ ] Teste com cliente real (pulado sem `ZB_CLIENT`): as flags de colisão lidas de um tile batem com o padrão da classe quando a propriedade não vem gravada (pelo menos uma mesh que bloqueia e uma que não bloqueia)
- [ ] Conferência da proporção humano × monstro registrada nas notas
