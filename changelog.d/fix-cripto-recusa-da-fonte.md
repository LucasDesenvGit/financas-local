### Correções
- **A cotação de criptomoedas volta a atualizar, agora pela Binance** (#762). Desde 14/11 o Mercado Bitcoin recusava as consultas do site, e o preço do Bitcoin e das outras moedas ficou parado. A cotação passa a vir da Binance, também em reais. Se a fonte recusar de novo, Configurações › Erros mostra um erro só, dizendo que não é falta de internet.
  - Técnico: saída de rede `api.binance.com` no lugar de `api.mercadobitcoin.net` (ADR 0073, substitui a 0048); `cripto.ErrRecusa` para 4xx (menos 408 e 429).
