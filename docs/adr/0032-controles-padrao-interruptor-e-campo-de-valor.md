# 0032. Controles padrão: interruptor, caixa de marcar, campo de valor, campo de data e lista suspensa

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

## Caixa de marcar e seleção em tabelas (#133)

- **Decisão:** a caixa de marcar ganha visual próprio **em todo o site**, em tabelas e formulários, com CSS sobre o mesmo `<input type="checkbox">` nativo (`appearance: none`). São 18 px, borda em `--texto-suave` (≥ 3:1, WCAG 1.4.11), marcada em `--primaria` com o visto e parcial (`indeterminate`) com um traço. O interruptor (`role="switch"`) fica de fora. A #133 previa manter as caixas de formulário com o visual do navegador; a decisão de aplicar em todo o site foi **do usuário**, por consistência (Nielsen 4).
- **Seleção em tabelas:**
  - a célula inteira é o alvo (`label.celula-marcar`, ≥ 24 px) e a linha marcada fica destacada (`tr.selecionada`);
  - o nome da caixa diz a linha ("Selecionar: descrição, data, valor", respeitando "ocultar valores");
  - em Transações também há Shift+clique (intervalo), "todas desta página" com estado parcial, Esc para limpar, contagem com "M em outras páginas" (aria-live) e barra de ações que abre animando e fica fixa no topo.
- **Seleção ao trocar de página × filtro** (*julgamento próprio*, como a issue sugeria): trocar de página **mantém** a seleção; trocar o filtro **limpa**, com aviso, para nenhuma ação em lote pegar transação fora do filtro.
- **Negativas:**
  - o visto e o traço são desenhados com `::after`, e o visual depende do CSS do site nos dois temas;
  - "selecionar todas as N do filtro" (opcional na #133) ficou de fora: pediria à API os ids do filtro.

## Campo de data, lista suspensa e seletor de mês (pedido do usuário durante a revisão do PR)

- **Campo de data:** `app-campo-data` (`compartilhado/campo-data.component.ts`) no lugar de todo `<input type="date">`.
  - **Por que:** o campo nativo aceitava ano de 6 dígitos ("13/02/275760"), e o calendário era o do navegador, fora do visual do site.
  - **Como funciona:** digita-se dd/mm/aaaa, com as barras entrando sozinhas e o ano sempre com 4 dígitos. Colar `AAAA-MM-DD` também funciona.
  - **Calendário próprio:** segue o padrão *Date Picker Dialog* do WAI-ARIA APG (setas, Home/End, PageUp/PageDown, Shift para o ano, Esc), com « ‹ › », "Hoje" e "Limpar".
  - **Limites:** 1900–2099 por padrão, ou os da tela (`[maximo]="hoje"`).
  - **Valor:** entrega o mesmo `'AAAA-MM-DD'` ao `ngModel` (`''` quando vazio). `(alterado)` faz o papel do `(change)` nativo.
  - **Alternativas descartadas:** só limitar o campo nativo com `max` (resolvia o ano, mas não o visual) e uma biblioteca de calendário (dependência externa).
- **Lista suspensa:** a `ListaSuspensaDirective` (`compartilhado/lista-suspensa.directive.ts`) mostra, no computador, a lista aberta do site por cima do `<select>` nativo, que continua dono do valor (`ngModel`, `[ngValue]`, `(change)`). No toque fica a lista do sistema. O campo fechado tem a seta desenhada no CSS.
  - **Descartado:** `appearance: base-select`, porque no Edge 155 o texto escolhido passava por baixo da seta.
- **Seletor de mês:** o nome do mês abre um quadro com o ano e os 12 meses, para pular vários meses de uma vez.
- **Negativas:**
  - mais código próprio de controle (teclado, posição, foco) para manter, coberto por testes em `testes/usabilidade/calendario.spec.ts`, `lista-suspensa-aberta.spec.ts` e `seletor-mes.spec.ts`;
  - o calendário e a lista têm de ser conferidos nos dois temas a cada mudança de cor;
  - no celular, a lista suspensa continua a do sistema (escolha, *julgamento próprio*).
- **Na prática:**
  - campo de data novo → `<app-campo-data name="data" [(ngModel)]="fm.data" obrigatorio />`;
  - tela nova com `<select>` → importar a `ListaSuspensaDirective`;
  - dois testes falham se uma tela usar `type="date"` ou um `<select>` sem a diretiva.

## Contexto (campo de valor, #132)

Os campos de dinheiro eram `<input type="number">` puros, cada tela com seu `min`/`step`. O campo numérico do navegador não entende o jeito brasileiro: "1.500" podia virar 1,5, colar "R$ 1.234,56" do app do banco deixava o campo vazio, a rodinha do mouse mudava o valor sem querer e as setinhas somavam 0,01.

## Alternativas consideradas (campo de valor)

- **Biblioteca de máscara** — resolve a formatação ao digitar, mas é dependência externa para um caso só.
- **Manter `type="number"` com `lang="pt-BR"`** — o comportamento com vírgula e ponto varia por navegador e continua sem aceitar o "R$" colado.
- **Componente próprio `app-campo-valor`** com `<input type="text" inputmode="decimal">`, que formata enquanto se digita e lê do jeito completo o que é colado.

## Decisão (campo de valor)

Todo campo de **dinheiro** usa o **`app-campo-valor`** (`compartilhado/campo-valor.component.ts`), um `ControlValueAccessor` + `Validator` que entrega ao `ngModel` um `number` (ou `null`), como antes — **a API não muda**.

- Lê "1234,56", "1.234,56", "R$ 1.234,56" e "1234.56". Regra do ponto sozinho (*julgamento próprio*): um ponto seguido de exatamente 3 dígitos é milhar ("1.500" = 1500); com outra quantidade é decimal.
- **Formata enquanto se digita:** os pontos de milhar entram a cada tecla ("124214" vira "124.214"), sem tirar o cursor do lugar (a posição é contada pelos dígitos). Um ponto digitado vira a vírgula decimal, e letras são ignoradas. Backspace/Delete sobre um ponto de milhar apaga o dígito ao lado. **Colar** (ou trocar o texto todo de uma vez) usa a leitura completa. Ao **sair**, completa as casas ("124.214,00").
- Parâmetros: `moeda` (símbolo dentro do campo, decorativo; o nome acessível diz "em reais"/"em dólares"), `casas` (máximo de casas, mínimo 2; cotação usa 4), `minimo`, `obrigatorio`, `negativo` (saldo inicial, aporte), `compacto` e `rotulo` (célula de tabela), `confirmado` + `enterConfirma` (valor salvo na própria linha, como no orçamento).
- Erro em texto perto do campo, com `aria-invalid` e `aria-describedby`, só depois de sair do campo (WCAG 3.3.1). O rótulo visível continua (WCAG 3.3.2); o "(R$)" sai do rótulo porque passa para dentro do campo.
- **Ficam como estão:** quantidade de cotas, câmbio, parâmetros da Parametrização (nem todos são dinheiro), dias e o campo da Sugestão de orçamentos (que esconde o valor com "ocultar valores"). "Ocultar valores" não vale nos campos de edição: quem edita precisa ver o número (*julgamento próprio*).

## Consequências (campo de valor)

- **Positivas:** o mesmo campo em todas as telas; colar do banco funciona; nada de valor mudado pela rodinha; os valores salvos na API são os mesmos números.
- **Negativas:**
  - a regra do ponto sozinho tem um caso ambíguo: "12.345" vira doze mil, mesmo numa cotação em dólar que fosse 12,345 — quem digita com vírgula não é afetado;
  - a conta dentro do campo ("120+35,50"), opcional na issue, ficou de fora;
  - o componente desenha a "caixa" do campo (fundo, borda, foco) por fora do `<input>`: mudança no visual dos campos precisa olhar os dois;
  - formatar ao digitar exige controlar o cursor à mão (`formatarDigitando`): um caso de edição não previsto pode deixar o cursor num lugar estranho; os testes cobrem digitar no fim e no meio, apagar o ponto de milhar e colar.
- **Na prática:** campo de dinheiro novo → `<label>Valor <app-campo-valor name="valor" [(ngModel)]="fm.valor" obrigatorio [minimo]="0.01" /></label>`. Testes em `testes/regressao/campo-valor.spec.ts` (inclui uma trava que falha se um campo de dinheiro voltar a ser `type="number"`) e `testes/usabilidade/campo-valor.spec.ts`.
