### Correções
- **Cotação de cripto: a recusa do Mercado Bitcoin aparece como erro, uma vez** (#762). Desde 14/11 o Mercado Bitcoin recusa as consultas, e Configurações › Erros mostrava um aviso de internet a cada 30 minutos. Agora aparece um erro só, dizendo que a fonte recusou o pedido e que não é falta de internet.
  - Técnico: `cripto.ErrRecusa` para 4xx (menos 408 e 429), separado do `ErrRede`; ADR 0048 atualizada.
