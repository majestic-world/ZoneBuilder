# 04: Formato UE2HUM01, pose e skinning

**What to build:** o módulo `model` decodifica e recodifica o formato `UE2HUM01` exatamente como o `bundle.rs` do UE2-Studio o define (`Decode([]byte) (*Bundle, error)` e `Encode(*Bundle) []byte`), amostra a pose de um clip num instante (trilhas por frame, interpolação, composição da paleta com o inverse bind), faz a transição entre 2 clips e aplica o skinning na CPU, devolvendo posições e normais em coordenadas do modelo (spec D6). O `human.bin` do UE2-Studio é copiado sem alterar nenhum byte e embutido com `//go:embed`, com um README de procedência. Nada do cliente é lido em runtime.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] Teste do seam: decodificar e recodificar o `human.bin` embutido dá os mesmos bytes
- [ ] Teste do seam: um buffer truncado e um buffer com bytes sobrando são recusados
- [ ] Teste do seam: o `human.bin` posado no frame 0 do idle tem o menor Z dos vértices em 0 (±0,5) e altura 80 (±1)
- [ ] Teste do seam: um bundle sintético de 2 ossos com rotação conhecida no frame 1 leva um vértice ao ponto esperado, e a transição na metade fica entre as 2 poses
- [ ] O `human.bin` embutido é idêntico ao do UE2-Studio (hash registrado no README de procedência)
