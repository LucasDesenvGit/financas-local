# Componentes que já existem

Antes de fazer um gráfico, um campo, um filtro, um balão ou um modal, procure aqui: o padrão já tem componente, com a acessibilidade e os testes feitos. Refazer um deles numa tela quebra a consistência (Nielsen 4) e perde o que o componente já resolve. O código é a fonte: o comentário no topo de cada arquivo diz como usar, e esta tabela só diz **qual** usar.

Todos em `src/app/compartilhado/`, numa subpasta (`ui/`, `graficos/`, `bancos/`, `lancamentos/`, `glossario/`; ADR 0058), salvo indicação.

## Gráficos e telas densas (ADR 0021)

| Preciso de… | Use | O que ele já faz |
|---|---|---|
| Qualquer gráfico | `<app-grafico [configuracao]="...">` (`grafico.component.ts`) | Chart.js em `<canvas>` (cabe na CSP), cores do tema, "ocultar valores", `dicaClique` e `itemClicavel` |
| Gráfico de linha | o mesmo, com `type: 'line'` | `linhasNoPadrao` (`grafico-linhas.ts`): o padrão "gráfico de linha" de `docs/UI-UX.md` (sem `fill: true`, e como fugir dele) |
| Barras com linha | o mesmo, misto | `linhaPorCimaDasBarras` (`grafico-misto.ts`): o padrão "barras com linha" de `docs/UI-UX.md` (e a linha de referência) |
| A frase-resumo e o "como ler" | `<app-grafico-explicado [resumo]="..." comoLer="...">` envolvendo o `<app-grafico>` | o padrão "gráfico se explica" de `docs/UI-UX.md` (WCAG 1.1.1) |
| Cor com significado | `CORES_GRAFICO` (`cores-graficos.ts`) | receita verde, despesa rosa, emergência laranja, em todas as telas |
| Rosca de categorias | `rosca-categorias.ts` | junta as fatias menores que 3% em "Outras" (WCAG 2.5.8) |
| Clique no gráfico | `(itemClicado)` **mais** um `<a>`/`<button>` com a mesma ação no HTML | WCAG 2.1.1 em `docs/UI-UX.md` |
| Esconder análises de apoio | `<app-mais-analises chave="<tela>" conteudo="o que tem dentro">` com `<ng-template>` | o padrão "tela densa" de `docs/UI-UX.md`; a escolha fica em `financas.maisAnalises.<tela>` |

## Campos e controles (ADR 0032)

| Preciso de… | Use | O que ele já faz |
|---|---|---|
| Valor em dinheiro | `<app-campo-valor>` (`campo-valor.component.ts`) | lê "1.234,56", "R$ 1.234,56" colado do banco, "1234.56"; `ControlValueAccessor` e validador (`minimo`, `maximo`, `obrigatorio`); funciona com `ngModel` |
| Data | `<app-campo-data>` (`campo-data.component.ts`) | calendário do site com `minimo`/`maximo`, título que sobe de nível (dia → mês → ano) e Esc que volta um nível; `ControlValueAccessor` |
| Mês | `<app-seletor-mes [valor]="..." (mudou)="...">` (`seletor-mes.component.ts`) | ‹ › e o quadro de meses com o ano; mesmo padrão de subir de nível (#342) |
| Escolher vários | `<app-seletor-multiplo>` (`seletor-multiplo.component.ts`) | no lugar do `<select multiple>`: caixas de marcar, resumo no botão ("3 categorias") |
| Lista suspensa | `<select>` normal **com** a `ListaSuspensaDirective` nos `imports` (`lista-suspensa.directive.ts`, seletor `select:not([multiple])`) | a lista no visual do site, por cima do nativo, que continua dono do valor |
| Interruptor (efeito imediato) | `<label class="interruptor"><input type="checkbox" role="switch" ...> Rótulo</label>` | o desenho está no `styles.css`; teclado e leitor de tela vêm do checkbox nativo. Quando usar: `docs/UI-UX.md` e ADR 0032 |
| Marcar linha de tabela | `<td><label class="celula-marcar"><input type="checkbox" [attr.aria-label]="'... ' + nome"></label></td>` e `[class.selecionada]` no `<tr>` | o padrão "seleção em tabela" de `docs/UI-UX.md` |
| Janela por cima | `<app-modal [titulo]="..." [aberto]="..." (fechar)="...">` (`modal.component.ts`) | o último aberto fica por cima e o Esc fecha só ele. Prefira navegar para a tela de detalhe quando ela existe (ADR 0018) |
| Perguntar antes de agir (excluir, descartar) | `await inject(ConfirmacaoService).confirmar({ mensagem, confirmar: '<verbo>', perigo })` (`confirmacao.service.ts`; o `<app-confirmacao>` já está no `layout`) | nunca `confirm()` ([ADR 0057](../../../../docs/adr/0057-confirmacao-pelo-modal-do-site-e-nao-pelo-confirm.md)); no teste, espie o `confirmar` do serviço |
| Menu de ações | `<app-menu-acoes>` (`menu-acoes.component.ts`) | padrão Menu Button do WAI-ARIA: setas, Home/End, Esc devolve o foco |
| Paginação | `<app-paginacao>` (`paginacao.component.ts`) | primeira/última, janela de números com reticências, "ir para" e itens por página |

## Texto, ajuda e avisos

| Preciso de… | Use | O que ele já faz |
|---|---|---|
| Termo técnico com definição | `<app-termo chave="...">texto</app-termo>` (`termo.component.ts`), com a definição em `glossario.ts` | ⓘ que abre por clique, toque e teclado, balão na top layer (ADR 0020). Termo novo: uma entrada no glossário, nunca o texto na tela |
| Dica de botão só com ícone | atributo `appDica="..."` (`dica.directive.ts`) | aparece no hover **e** no foco, some com Esc (WCAG 1.4.13). Nunca só `title=` |
| Aviso depois de uma ação | `inject(AvisosService)`: `.sucesso(texto, acao?)`, `.erro(texto)`, `.info(texto)` (`core/avisos.service.ts`; o `<app-avisos>` do `ui/` já fica no `app.component`) | aviso com "Desfazer" fica mais tempo |
| Dinheiro no texto | `{{ valor \| currency }}` com o `MoedaPipe` nos `imports` (`moeda.pipe.ts`) | "ocultar valores" e o "−" de verdade. **Não** importe o `CurrencyPipe` do Angular |
| Ícone | `<app-icone [icone]="Icones.x">` (`icone.component.ts`), com os ícones de `Icones` em `icones.ts` | Lucide empacotado no build (ADR 0016); no Jest vira o `lucide-falso.js` |
| Ícone da categoria, selo do banco, selo do serviço | `<app-icone-categoria>`, `<app-logo-banco>` (ADR 0024), `<app-selo-servico>` (#265) | decorativos: o nome vem escrito ao lado (WCAG 1.1.1) |
| Link para Transações filtrada | `paramsTransacoes({ categoriaId, inicio, fim, tipo })` (`utilidades.ts`) | o contexto completo do detalhe por navegação (ADR 0018); nunca monte a query string na mão |
| Tipo de lançamento em português | `tipos-lancamento.ts` | "Pix enviado", não "DEBIT" (Nielsen 2; ADR 0025) |

## Componente novo em `compartilhado/`

Só quando uma segunda tela precisar (antes disso, fica na pasta da tela; se só o `shell` usa, em `layout/`). Vai para `ui/` se não sabe nada do domínio, senão para a pasta do assunto (ADR 0058). Ele não importa `paginas/` nem `layout/` (teste de arquitetura), traz o comentário no topo com o princípio de UI/UX e um exemplo de uso, e ganha teste em `testes/jest/usabilidade/` com o critério que ele garante. Padrão novo de UI entra na lista "padrões já adotados" de `docs/UI-UX.md`, com ADR se mudar o que já foi decidido.
