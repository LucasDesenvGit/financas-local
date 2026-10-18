# 0032. Controles padrão: interruptor para efeito imediato e campo de valor em reais

- **Status:** Aceita
- **Data:** 2026-10-18
- **Relacionadas:** [0018](0018-estado-da-tela-na-url-e-detalhe-por-navegacao.md) (estado na URL), [0021](0021-componentes-padrao-para-graficos-e-telas-densas.md) (componentes padrão)

## Contexto

Dois tipos de controle apareciam de jeitos diferentes em cada tela:

- **Liga/desliga com efeito imediato** ("mostrar arquivadas" em Contas e cartões, "mostrar encerradas" em Investimentos, "agrupar subcategorias" em Relatórios). Era uma caixa de marcar solta, pequena, em minúsculas, igual às de formulário que esperam um "Salvar" (#130).
- **Valor em dinheiro** (ver a segunda parte, #132).

## Alternativas consideradas (interruptor)

- **Componente Angular `app-interruptor`** — encapsula o visual. Contra: mais código, e teclado, `ngModel` e leitor de tela teriam de ser refeitos ou repassados.
- **CSS sobre o `<input type="checkbox">` nativo, com `role="switch"`** — o input continua o mesmo: Espaço alterna, `ngModel`/`[checked]` funcionam e o leitor de tela anuncia "interruptor, ligado/desligado". Só a aparência muda.
- **Manter a caixa de marcar** — não distingue "muda na hora" de "muda ao salvar" (NN/g, *Toggle-Switch Guidelines*).

## Decisão (interruptor)

Usamos o estilo **`label.interruptor`** em `styles.css` sobre o checkbox nativo com **`role="switch"`**, só onde o efeito é **imediato**. O que depende de "Salvar" continua caixa de marcar (`label.checkbox`).

- Rótulo curto, palavra-chave primeiro, que **não muda** com o estado ("Mostrar arquivadas", nunca "Esconder arquivadas"; WAI-ARIA APG, *Switch*).
- Trilho de 40×24 px e rótulo clicável (alvo ≥ 24 px, WCAG 2.5.8); foco visível; animação curta da bolinha, desligada com `prefers-reduced-motion`.
- "Mostrar arquivadas" guarda o estado na URL (`?arquivadas=1`, com `replaceUrl`, ADR 0018), e cada conta ou cartão arquivado mostra o selo **"Arquivada"** em texto, além do visual apagado (WCAG 1.4.1).
- **Filtros de Transações** ("somente emergências", "descrição pouco informativa") **continuam caixa de marcar**: ficam na barra de filtros, ao lado de campos de data, busca e listas, e mudar só esses dois para interruptor quebraria a consistência visual da barra (Nielsen 4). *Julgamento próprio*: na barra de filtros todo controle já tem efeito imediato, então a caixa de marcar não engana ali como enganava sozinha no cabeçalho de Contas.

## Consequências (interruptor)

- **Positivas:** o mesmo controle significa sempre "muda na hora"; nenhum código de acessibilidade novo (vem do input nativo).
- **Negativas:**
  - `appearance: none` desenha o controle à mão: o visual depende do CSS do site e precisa ser conferido nos dois temas a cada mudança de tokens de cor;
  - fica a cargo de quem cria uma tela escolher certo entre interruptor e caixa de marcar: um interruptor num formulário com "Salvar" engana.
- **Na prática:** efeito imediato → `<label class="interruptor"><input type="checkbox" role="switch" …> Rótulo</label>`; formulário com "Salvar" → `label.checkbox`. Teste em `testes/usabilidade/interruptor.spec.ts`.
