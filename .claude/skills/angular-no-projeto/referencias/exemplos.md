# Exemplos no código

Antes de criar algo, abra o exemplo e siga a forma dele. Estes são **ponteiros**, não cópias: o código é a fonte e não envelhece aqui. Se um exemplo deixar de seguir o padrão, troque o ponteiro no mesmo PR. Caminhos a partir de `src/frontend`.

| Preciso de… | Olhe | O que copiar |
|---|---|---|
| Tela que responde primeiro à pergunta dela, com análises de apoio a um clique | `src/app/paginas/painel/painel.component.ts`, `anual.component.ts`, `investimentos-fiis.component.ts` | o bloco principal no topo; o resto em `<app-mais-analises chave="<tela>" conteudo="...">` |
| Estado da tela na URL e filtros em chips | `paginas/movimentacao/transacoes.component.ts` (filtros, `replaceUrl`, "Limpar filtros") e `paginas/ajustes/erros.component.ts` (menor) | ler os query params ao abrir; gravar com `replaceUrl: true`; chips com ✕ |
| Aba ou filtro lido da URL, tela pequena | `paginas/ajustes/configuracoes.component.ts`, `paginas/movimentacao/assinaturas-recorrentes.component.ts` | `rota.queryParamMap.pipe(takeUntilDestroyed()).subscribe(...)` num signal |
| Detalhe por navegação | `paginas/analises/relatorios.component.ts`, `paginas/planejamento/orcamentos.component.ts` | o total leva a `/transacoes` com `paramsTransacoes(...)` |
| Gráfico explicado | `paginas/analises/relatorios.component.ts`, `paginas/analises/projecao.component.ts` | `<app-grafico-explicado [resumo] comoLer>` em volta do `<app-grafico>`; em barras com linha, o valor de cada ponto também na tabela do bloco |
| Componente com todo o estado em signal | `paginas/patrimonio/rendimento-investimentos-of.component.ts` | `signal`/`computed`, resumo antes da tabela, a única com OnPush |
| Página grande com template e CSS em arquivo próprio | `paginas/movimentacao/transacoes.component.ts` com `.html` e `.css` (ADR 0059) | `templateUrl: './<tela>.component.html'`, `styleUrl: './<tela>.component.css'`; teste que procura no template lê pelo `testes/jest/apoio/codigo.ts` |
| Cálculo em função pura, com o teste | `paginas/movimentacao/transacoes.ts` (a lógica de uma página grande, ADR 0059; teste em `testes/jest/regressao/transacoes-logica.spec.ts`), `paginas/movimentacao/limite-cartao.ts` (teste em `testes/jest/regressao/limite-cartao.spec.ts`), `paginas/planejamento/metas-plano.ts`, `paginas/movimentacao/recorrentes.ts`, `paginas/movimentacao/conferir-banco.ts` | arquivo `.ts` ao lado da tela, sem Angular; o componente só chama |
| Interruptor e seleção em tabela | `paginas/movimentacao/contas.component.ts` (interruptores), `paginas/planejamento/sugestao-orcamentos.component.ts` (`celula-marcar`) | o HTML de `referencias/componentes.md` |
| Formulário em modal, com campo de valor e de data | `paginas/planejamento/metas.component.ts` (valor e data), `paginas/planejamento/orcamentos.component.ts` (só valor) | `<app-modal>`, `ngModel`, `<app-campo-valor>`, `<app-campo-data>` |
| Preferência lembrada no navegador | `compartilhado/graficos/mais-analises.component.ts`, `core/privacidade.service.ts`, `compartilhado/lancamentos/faturas-no-painel.ts` | chave `financas.*`, `try`/`catch` na leitura e na gravação |
| Componente compartilhado com teste de usabilidade | `compartilhado/graficos/mais-analises.component.ts` e `testes/jest/usabilidade/mais-analises.spec.ts` | comentário no topo com o princípio e o exemplo de uso; teste com componente hospedeiro |
| Fábrica de dados do Jest | `testes/jest/apoio/contas.ts` (`conta()`, `BANCOS`) | uma função por entidade, com `Partial<T>` para o que o teste muda |
| Teste de interação no navegador | `testes/navegador/seletor-mes.spec.ts`, `testes/navegador/celular.spec.ts` | `page.goto('/rota')`, `getByRole`/`getByLabel`, espera da troca antes de ler |
| Teste de estilo calculado | `testes/navegador/estilos.spec.ts` | medir o efeito (`boundingBox`), não procurar a regra no CSS |

**Não copie a estrutura de** `paginas/patrimonio/investimentos.component.ts` (1.415 linhas) nem de `paginas/planejamento/metas.component.ts` (992): os padrões delas valem, o tamanho não. Página grande se divide como diz a ADR 0059; `transacoes` é a primeira (#475), e a lógica dela já está em `transacoes.ts`.
