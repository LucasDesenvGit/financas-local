### Funcionalidades
- **Manual com imagens das telas** (#752). Cerca de 26 partes do Manual agora mostram uma imagem da tela, no tema claro ou escuro que você usa, entre o passo a passo e o botão "Abrir a tela". A busca do Manual também acha pela descrição da imagem.
  - Técnico: prints gerados pelo `npm run prints:manual` (Playwright, API falsa, WebP em `public/imagens/manual/`); o servidor passa a servir `.webp` (ADR 0071).
