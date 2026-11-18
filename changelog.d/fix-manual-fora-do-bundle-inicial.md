### Correções
- **O site abre mais leve: o texto do Manual só é baixado quando o Manual abre** (#755). Antes, toda página baixava o Manual inteiro junto com o menu.
  - Técnico: `ajudaDaUrl` e `AJUDA_POR_ROTA` saem do `manual-conteudo.ts` para `paginas/manual/ajuda-por-rota.ts`; o `main` cai de 480,0 para 356,4 kB.
