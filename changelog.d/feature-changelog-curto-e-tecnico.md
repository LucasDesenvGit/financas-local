### Publicação e documentação
- **Notas das versões mais curtas** (#722). Cada entrada passa a dizer só o que muda para quem usa, em poucas palavras. O detalhe técnico fica à parte, e a janela "O que mudou" do gerenciador e as notas dos Releases não o mostram.
  - Técnico: sub-item `  - Técnico: ...` (uma linha) em `changelog.d/`; escondido por `HTMLDasNotas` (gerenciador) e `notas.sh`, e mantido no `CHANGELOG.md`. O `verificar-changelog.py` avisa (`::warning`, sem barrar) acima de ~60 palavras fora do "Técnico".
