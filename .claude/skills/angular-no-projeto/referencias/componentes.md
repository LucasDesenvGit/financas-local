# Componentes que já existem

Antes de fazer um gráfico, um campo, um filtro, um balão ou um modal, procure aqui: o padrão já tem componente, com a acessibilidade e os testes feitos. Refazer um deles numa tela quebra a consistência (Nielsen 4) e perde o que o componente já resolve. O código é a fonte: o comentário no topo de cada arquivo diz como usar, e esta tabela só diz **qual** usar.

Todos em `src/app/compartilhado/`, salvo indicação.

## Gráficos e telas densas (ADR 0021)

| Preciso de… | Use | O que ele já faz |
|---|---|---|
| Qualquer gráfico | `<app-grafico [configuracao]="...">` (`grafico.component.ts`) | Chart.js em `<canvas>` (cabe na CSP), cores do tema, "ocultar valores", `dicaClique` e `itemClicavel` |
| Gráfico de linha | o mesmo, com `type: 'line'` | `linhasNoPadrao` (`grafico-linhas.ts`): a principal preenchida até a base, com pontos; a tracejada sem nada. **Não use `fill: true`**; para fugir do padrão, o conjunto declara `fill: false` ou `pointRadius` |
| Barras com linha | o mesmo, misto | `linhaPorCimaDasBarras` (`grafico-misto.ts`): a linha por cima, um ponto por mês. Linha de referência (média, meta) declara `pointRadius: 0` |
| A frase-resumo e o "como ler" | `<app-grafico-explicado [resumo]="..." comoLer="...">` envolvendo o `<app-grafico>` | o padrão "gráfico se explica" do `CLAUDE.md`; o resumo é a alternativa em texto do `<canvas>` (WCAG 1.1.1) e respeita "ocultar valores" |
| Cor com significado | `CORES_GRAFICO` (`cores-graficos.ts`) | receita verde, despesa rosa, emergência laranja, em todas as telas |
| Rosca de categorias | `rosca-categorias.ts` | junta as fatias menores que 3% em "Outras" (WCAG 2.5.8) |
| Clique no gráfico | `(itemClicado)` **mais** um `<a>`/`<button>` com a mesma ação no HTML | clique no `<canvas>` nunca é o único caminho (WCAG 2.1.1; `CLAUDE.md`) |
| Esconder análises de apoio | `<app-mais-analises chave="<tela>" conteudo="o que tem dentro">` com `<ng-template>` | botão com `aria-expanded`, conteúdo criado só ao abrir, escolha lembrada por tela em `financas.maisAnalises.<tela>` (Progressive Disclosure) |

## Campos e controles (ADR 0032)

| Preciso de… | Use | O que ele já faz |
|---|---|---|
| Valor em dinheiro | `<app-campo-valor>` (`campo-valor.component.ts`) | lê "1.234,56", "R$ 1.234,56" colado do banco, "1234.56"; `ControlValueAccessor` e validador (`minimo`, `maximo`, `obrigatorio`); funciona com `ngModel` |
| Data | `<app-campo-data>` (`campo-data.component.ts`) | calendário do site com `minimo`/`maximo`, título que sobe de nível (dia → mês → ano) e Esc que volta um nível; `ControlValueAccessor` |
| Mês | `<app-seletor-mes [valor]="..." (mudou)="...">` (`seletor-mes.component.ts`) | ‹ › e o quadro de meses com o ano; mesmo padrão de subir de nível (#342) |
| Escolher vários | `<app-seletor-multiplo>` (`seletor-multiplo.component.ts`) | no lugar do `<select multiple>`: caixas de marcar, resumo no botão ("3 categorias") |
| Lista suspensa | `<select>` normal **com** a `ListaSuspensaDirective` nos `imports` (`lista-suspensa.directive.ts`, seletor `select:not([multiple])`) | a lista no visual do site, por cima do nativo, que continua dono do valor |
| Interruptor (efeito imediato) | `<label class="interruptor"><input type="checkbox" role="switch" ...> Rótulo</label>` | o desenho está no `styles.css`; teclado e leitor de tela vêm do checkbox nativo. O que depende de "Salvar" é caixa de marcar, não interruptor |
| Marcar linha de tabela | `<td><label class="celula-marcar"><input type="checkbox" [attr.aria-label]="'... ' + nome"></label></td>` e `[class.selecionada]` no `<tr>` | a célula inteira é o alvo; o nome da caixa diz qual linha |
| Janela por cima | `<app-modal [titulo]="..." [aberto]="..." (fechar)="...">` (`modal.component.ts`) | o último aberto fica por cima e o Esc fecha só ele. Prefira navegar para a tela de detalhe quando ela existe (ADR 0018) |
| Menu de ações | `<app-menu-acoes>` (`menu-acoes.component.ts`) | padrão Menu Button do WAI-ARIA: setas, Home/End, Esc devolve o foco |
| Paginação | `<app-paginacao>` (`paginacao.component.ts`) | primeira/última, janela de números com reticências, "ir para" e itens por página |

## Texto, ajuda e avisos

| Preciso de… | Use | O que ele já faz |
|---|---|---|
| Termo técnico com definição | `<app-termo chave="...">texto</app-termo>` (`termo.component.ts`), com a definição em `glossario.ts` | ⓘ que abre por clique, toque e teclado, balão na top layer (ADR 0020). Termo novo: uma entrada no glossário, nunca o texto na tela |
| Dica de botão só com ícone | atributo `appDica="..."` (`dica.directive.ts`) | aparece no hover **e** no foco, some com Esc (WCAG 1.4.13). Nunca só `title=` |
| Aviso depois de uma ação | `inject(AvisosService)`: `.sucesso(texto, acao?)`, `.erro(texto)`, `.info(texto)` (`avisos.component.ts`) | aviso com "Desfazer" fica mais tempo |
| Dinheiro no texto | `{{ valor \| currency }}` com o `MoedaPipe` nos `imports` (`moeda.pipe.ts`) | "ocultar valores" e o "−" de verdade. **Não** importe o `CurrencyPipe` do Angular |
| Ícone | `<app-icone [icone]="Icones.x">` (`icone.component.ts`), com os ícones de `Icones` em `icones.ts` | Lucide empacotado no build (ADR 0016); no Jest vira o `lucide-falso.js` |
| Ícone da categoria, selo do banco, selo do serviço | `<app-icone-categoria>`, `<app-logo-banco>` (ADR 0024), `<app-selo-servico>` (#265) | decorativos: o nome vem escrito ao lado (WCAG 1.1.1) |
| Link para Transações filtrada | `paramsTransacoes({ categoriaId, inicio, fim, tipo })` (`utilidades.ts`) | o contexto completo do detalhe por navegação (ADR 0018); nunca monte a query string na mão |
| Tipo de lançamento em português | `tipos-lancamento.ts` | "Pix enviado", não "DEBIT" (Nielsen 2; ADR 0025) |

## Componente novo em `compartilhado/`

Só quando uma segunda tela precisar (antes disso, fica na tela). Ele não importa `paginas/` nem `layout/` (teste de arquitetura), traz o comentário no topo com o princípio de UI/UX e um exemplo de uso, e ganha teste em `testes/usabilidade/` com o critério que ele garante. Padrão novo de UI entra na lista "padrões já adotados" do `CLAUDE.md`, com ADR se mudar o que já foi decidido.
