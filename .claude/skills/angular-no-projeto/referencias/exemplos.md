# Exemplos no código

Antes de criar algo, abra o exemplo e siga a forma dele. Estes são **ponteiros**, não cópias: o código é a fonte e não envelhece aqui. Se um exemplo deixar de seguir o padrão, troque o ponteiro no mesmo PR. Caminhos a partir de `src/frontend`.

| Preciso de… | Olhe | O que copiar |
|---|---|---|
| Tela que responde primeiro à pergunta dela, com análises de apoio a um clique | `src/app/paginas/painel.component.ts`, `anual.component.ts`, `investimentos-fiis.component.ts` | o bloco principal no topo; o resto em `<app-mais-analises chave="<tela>" conteudo="...">` |
| Estado da tela na URL e filtros em chips | `paginas/transacoes.component.ts` (filtros, `replaceUrl`, "Limpar filtros") e `paginas/erros.component.ts` (menor) | ler os query params ao abrir; gravar com `replaceUrl: true`; chips com ✕ |
| Aba ou filtro lido da URL, tela pequena | `paginas/configuracoes.component.ts`, `paginas/assinaturas-recorrentes.component.ts` | `rota.queryParamMap.pipe(takeUntilDestroyed()).subscribe(...)` num signal |
| Detalhe por navegação | `paginas/relatorios.component.ts`, `paginas/orcamentos.component.ts` | o total leva a `/transacoes` com `paramsTransacoes(...)` |
| Gráfico explicado | `paginas/relatorios.component.ts`, `paginas/projecao.component.ts` | `<app-grafico-explicado [resumo] comoLer>` em volta do `<app-grafico>`; em barras com linha, o valor de cada ponto também na tabela do bloco |
| Componente com todo o estado em signal | `paginas/rendimento-investimentos-of.component.ts` | `signal`/`computed`, resumo antes da tabela, a única com OnPush |
| Cálculo em função pura, com o teste | `paginas/limite-cartao.ts` (teste em `testes/regressao/limite-cartao.spec.ts`), `paginas/metas-plano.ts`, `paginas/recorrentes.ts`, `paginas/conferir-banco.ts` | arquivo `.ts` ao lado da tela, sem Angular; o componente só chama |
| Interruptor e seleção em tabela | `paginas/contas.component.ts` (interruptores), `paginas/sugestao-orcamentos.component.ts` (`celula-marcar`) | o HTML de `referencias/componentes.md` |
| Formulário em modal, com campo de valor e de data | `paginas/metas.component.ts` (valor e data), `paginas/orcamentos.component.ts` (só valor) | `<app-modal>`, `ngModel`, `<app-campo-valor>`, `<app-campo-data>` |
| Preferência lembrada no navegador | `compartilhado/mais-analises.component.ts`, `core/privacidade.service.ts`, `compartilhado/faturas-no-painel.ts` | chave `financas.*`, `try`/`catch` na leitura e na gravação |
| Componente compartilhado com teste de usabilidade | `compartilhado/mais-analises.component.ts` e `testes/usabilidade/mais-analises.spec.ts` | comentário no topo com o princípio e o exemplo de uso; teste com componente hospedeiro |
| Fábrica de dados do Jest | `testes/apoio/contas.ts` (`conta()`, `BANCOS`) | uma função por entidade, com `Partial<T>` para o que o teste muda |
| Teste de interação no navegador | `testes-navegador/seletor-mes.spec.ts`, `testes-navegador/celular.spec.ts` | `page.goto('/rota')`, `getByRole`/`getByLabel`, espera da troca antes de ler |
| Teste de estilo calculado | `testes-navegador/estilos.spec.ts` | medir o efeito (`boundingBox`), não procurar a regra no CSS |

**Não copie a estrutura de** `paginas/transacoes.component.ts` (1.630 linhas) nem de `paginas/investimentos.component.ts` (1.409): os padrões delas valem, o tamanho não. A divisão das páginas grandes está na #475.
