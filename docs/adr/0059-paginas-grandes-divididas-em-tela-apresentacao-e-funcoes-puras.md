# 0059. Páginas grandes do front-end divididas em componente da tela, componentes de apresentação e funções puras

- **Status:** Aceita
- **Data:** 2026-11-05
- **Relacionadas:** [0013](0013-testes-do-front-end-com-jest.md) (Jest, testes rápidos, #204); [0018](0018-estado-da-tela-na-url-e-detalhe-por-navegacao.md) (estado na URL); [0021](0021-componentes-padrao-para-graficos-e-telas-densas.md) (componentes padrão); [0047](0047-ci-paralela-so-testes-afetados-no-pr.md) (testes afetados no PR); [0058](0058-pastas-do-front-end-por-grupo-do-menu-e-por-assunto.md) (pastas)

## Contexto

As páginas grandes do front-end juntavam num arquivo só o template inline, o CSS e toda a lógica. Em 05/11/2026: `transacoes.component.ts` com 1.642 linhas (template de 440, CSS de 80 e uns 60 métodos), `investimentos` com 1.415, `metas` com 992 e `contas` com 850 (#475).

- Para testar uma regra dessas telas no Jest, era preciso montar o componente inteiro com o `TestBed`, a API falsa e o roteador, o que é lento e frágil. O Jest já chegou a 6 minutos (#204).
- Já havia bons exemplos do caminho: as funções puras ao lado da tela (`limite-cartao.ts`, `metas-plano.ts`, `recorrentes.ts`), os componentes de `compartilhado/` que recebem os dados e não chamam a API (gráfico, modal, paginação), e `grafico-linhas.ts` e `grafico-misto.ts`.
- O pedido da #475 cita o padrão MVP, que separa a tela (sem lógica) de um *presenter*.

## Alternativas consideradas

- **Deixar como está:** nada a mudar. Contra: as telas grandes só crescem, e cada regra nova só se testa pelo `TestBed`.
- **MVP "de livro"** (tela sem lógica nenhuma e um *presenter* à parte, que a tela chama): é o que a #475 cita. Contra: no Angular, o template se liga direto à classe do componente. Um *presenter* à parte vira um segundo componente sem template, ou um serviço por tela que repete o estado do componente. Forçar o padrão briga com o framework (*julgamento próprio*).
- **Estado num serviço ou numa store por tela** (NgRx, signal store): separa bem o estado. Contra: é dependência nova ou padrão novo para 4 telas, e o projeto não usa store (`referencias/estilo.md` da skill `angular-no-projeto`).
- **Três partes, do jeito do Angular** (escolhida): a página guarda o estado e fala com a API; componentes de apresentação desenham uma parte da tela; funções puras fazem a conta.

## Decisão

Página grande se divide em três partes.

- **Componente da tela** (a página, `paginas/<grupo>/<tela>.component.ts`): busca os dados pelo `ApiService`, guarda o estado e reage aos filtros e à URL (`replaceUrl`, ADR 0018). Os métodos que o template e os testes chamam continuam nele, mas, quando há regra, só chamam a função pura.
- **Componentes de apresentação** (`paginas/<grupo>/<tela>-<parte>.component.ts`, como `renomear-transacao.component.ts`): recebem por `input()` e avisam por `output()`; não chamam a API nem leem a rota; `ChangeDetectionStrategy.Eager`, como o resto do app. Passam a `compartilhado/` só quando outra tela também usa (ADR 0058).
- **Funções puras** (`paginas/<grupo>/<tela>.ts`, ao lado da página): cálculo, decisão, texto, agrupamento, filtro ↔ URL e montagem do que vai para a API. Sem Angular: a URL entra como `{ get(nome) }`, como o `ParamMap`, e um tipo de um componente só entra com `import type`. Os testes ficam em `testes/jest/regressao/`, sem `TestBed`.
- **Template e CSS em arquivo próprio** (`templateUrl`/`styleUrl`, o mesmo nome do `.ts`) quando o template passa de **150 linhas** (*julgamento próprio*: abaixo disso, ler tudo num arquivo só é mais fácil). Isso vale só depois do pré-requisito de "Na prática". Até lá, todo template continua inline.
- **Aos poucos:** `transacoes` primeiro, em PRs pequenos, uma parte da tela por PR. As outras páginas grandes seguem o padrão quando alguém mexe nelas, sem um PR gigante. Os testes do navegador da tela não mudam, porque o comportamento não muda.

## Consequências

- **Positivas:**
  - a regra se testa em milissegundos, sem montar a tela. Os 20 casos da `transacoes-logica.spec.ts` rodam em 0,5 s, contra 1,4 s dos 5 casos da `transacoes-ordenar.spec.ts`, que abre a tela pelo `TestBed` (medido em 05/11/2026, cada arquivo sozinho);
  - o componente da tela fica com o que só ele faz (estado, API, rota). O de Transações caiu de 1.642 para 1.286 linhas só com as funções puras.
- **Negativas:**
  - uma regra passa a morar em dois arquivos (o método do componente que chama e a função), e quem lê o componente precisa abrir a função para ver a regra;
  - os testes que já montam a tela continuam lá. Mover para a função pura um teste que já existe é trabalho à parte, e até lá alguns comportamentos podem ser testados nos dois lugares;
  - componente de apresentação com `input()`/`output()` pede passar o estado e os eventos de um lado para o outro. Numa tela com muito estado ligado (seleção, trocas pendentes, aviso ao sair), separar demais complica mais do que ajuda. Separe a parte que tem fronteira clara (um modal, uma barra), não a tabela inteira;
  - template em `.html` some de quem lê só o `.ts`: os testes que procuram nos templates como texto e o `testes-afetados.py` precisam passar a ler o `.html` antes da primeira tela com `templateUrl`, senão a verificação passa sem verificar nada.
- **Na prática:**
  - tela nova ou página grande que for mexida: regra nova vai para `<tela>.ts`, com teste no Jest sem `TestBed`; o método do componente só chama a função;
  - o teste de arquitetura (regra 7, `npm run test:arquitetura`, só local) confere que `paginas/<grupo>/<tela>.ts` não importa `@angular/*`, `rxjs` nem o `ApiService`. A regra do componente de apresentação (não importa o `ApiService`) entra com o primeiro deles, junto com um jeito de diferenciá-lo pelo nome;
  - **pré-requisito do `templateUrl`**, num PR antes da primeira tela que o usar:
    - os testes que leem o código das telas como texto, por exemplo `campo-valor.spec.ts`, `campo-data.spec.ts`, `clique-com-teclado.spec.ts`, `lista-suspensa-aberta.spec.ts` e `animacoes.spec.ts`, passam a ler também o `.html` (hoje só leem `.ts`, e o template fora dele passaria sem ser conferido);
    - o `scripts/ci/testes-afetados.py` liga `<tela>.component.html`/`.css` ao `.ts` da tela (hoje um `.html` em `src/app` roda todos os testes);
  - acompanhamento das telas: [#475](https://github.com/LucasDesenvGit/financas-local/issues/475).
