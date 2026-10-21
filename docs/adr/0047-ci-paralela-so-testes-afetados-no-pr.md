# 0047. CI em jobs paralelos e, no PR, só os testes afetados; .msi só quando o PR mexe nele

- **Status:** Aceita
- **Data:** 2026-10-21 (issue #255)
- **Relacionadas:** [0041](0041-testes-no-navegador-com-playwright.md) (testes no navegador); [0013](0013-testes-do-front-end-com-jest.md) (Jest); [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md) (o .msi na CI); [0039](0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md) (a versão sobe em todo PR)

## Contexto

Medido no PR #271, antes desta mudança (o tempo de um PR é o do job mais lento, porque os workflows rodam juntos):

| Job | Tempo | Onde vai |
|---|---|---|
| **gerar e testar o .msi** (Windows) | **~6–7 min** | gerar o .msi 139 s; duas instalações de teste 69 s + 69 s; instalar o .NET 35 s |
| build (Linux) | ~4,5 min | testes no navegador 113 s; Jest 47 s; build do front 34 s; API 18 s — **em fila, um depois do outro** |
| migrar da instalação pelos scripts | ~2 min | depois do .msi |

- O .msi devia rodar só quando o PR mexe no instalador (filtro `paths:` do `versao.yml`). Mas o `Directory.Build.props` está no filtro, e a versão sobe em **todo** PR (ADR 0039): o .msi rodava sempre, até num PR só de CSS.
- O `build` fazia tudo em sequência: API, front, Jest e navegador.
- Todo PR rodava a regressão inteira (178 testes no navegador, 86 arquivos do Jest), mesmo mudando uma tela.

## Alternativas consideradas

- **Etiqueta por tela em cada teste (`{ tag: '@transacoes' }`):** explícita, mas é mais uma regra a lembrar em todo teste novo, e teste sem etiqueta roda sempre ou nunca. Descartada em favor do grafo, que se mantém sozinho.
- **`--only-changed` do Playwright:** só enxerga mudança no próprio arquivo de teste (os testes abrem o build, não importam a tela). Não serve sozinho.
- **Imagem Docker oficial do Playwright** (navegador e bibliotecas do sistema já dentro): baixar a imagem (~1,5 GB) leva mais que os ~18 s de hoje com o navegador em cache. A CI não constrói imagem, então não há camada Docker para guardar. Descartada.
- **Tirar o `Directory.Build.props` do filtro do `versao.yml`:** perderia o .msi quando o props muda de verdade (ex.: um pacote). Descartada em favor de olhar o que mudou nele.
- **Dividir o navegador em mais partes (3, 4):** cada parte repete a preparação (~1 min). Com 2, o ganho já cobre; mais partes gastam mais minutos de CI quase sem ganho de relógio.

## Decisão

- **`ci.yml` em jobs paralelos:** "o que mudou" (decide), "API e banco", "front-end: Jest" e "front-end: navegador" em **2 partes** (`--shard`). O tempo do PR passa a ser o do mais lento, não a soma.
- **Um job final chamado `build`** junta os quatro: passa só se todos passaram ou foram pulados de propósito; falhou ou cancelou, falha. É o check que a proteção do `develop`/`main` exige pelo nome (o do job antigo): sem ele, o PR ficava esperando um `build` que não existia mais.
- **No PR, só os testes afetados** (`scripts/ci/testes-afetados.py`):
  - **navegador:** o arquivo que mudou em `src/app` afeta as telas (rotas do `app.routes.ts`) que o importam, direta ou indiretamente, pelo grafo de imports do próprio código. Rodam os arquivos de teste que abrem essas telas (`goto('/rota')`, `abrir(page, '/rota')`), as linhas do `telas.spec.ts` dessas rotas, o `celular.spec.ts` e os testes que mudaram (filtro `--grep` do Playwright, que confere o nome do arquivo e o título);
  - **Jest:** `--findRelatedTests` com os arquivos que mudaram (segue os imports, com `src/` nas `roots`), mais os testes que leem o código como texto (`readFileSync`/`readdirSync`, que o grafo não enxerga) e os testes que mudaram;
  - **tudo** quando muda arquivo global (`styles.css`, `core/`, `layout/`, rotas, `index.html`, configuração, apoio dos testes, a CSP do `Seguranca/`, a própria CI) ou arquivo do front-end sem regra. Na dúvida, tudo;
  - **nada** do front-end quando o PR não mexe nele (só API, docs, scripts): os jobs do front-end ficam de fora;
  - **no push do `develop`/`main`, sempre tudo.** O que escapar no PR aparece no merge, antes da versão.
- **.msi só quando o PR precisa** (`scripts/ci/precisa-msi.py`): instalador, gerenciador, inicialização da API, o próprio workflow, ou o `Directory.Build.props` **além da linha `<Version>`**. No push do `develop` e na tag, sempre.
- **Caches:** `node_modules` (pula o `npm ci`), build do Angular (`angular.json`: `cli.cache.environment: "all"`, porque o padrão desliga na CI), Jest (`--cacheDirectory`), NuGet (também no job do .msi, que publica self-contained para win-x64), e o Chromium do Playwright, como antes.

## Consequências

- **Positivas:**
  - PR que não mexe no instalador deixa de esperar ~7 min do .msi;
  - o front-end testa em paralelo com a API, e o navegador em duas máquinas;
  - PR de uma tela roda só os testes dela (ex.: Painel, 82 de 178 no navegador e 20 de 86 arquivos do Jest);
  - o porquê da seleção aparece no resumo de cada execução ("rodando /painel porque mudou paginas/painel-avisos.component.ts").
- **Negativas:**
  - **um teste afetado pode escapar no PR:** dependência que o grafo não vê (ex.: CSS de componente que muda outra tela por cascata, teste que abre a tela por um caminho sem a rota escrita). Mitigação: tudo no merge e arquivos globais disparam tudo;
  - **mais minutos de CI no total** (cada job prepara a própria máquina), em troca de menos tempo de relógio;
  - **cache que vale demais:** um `node_modules` ou build do Angular estragado no cache pode passar adiante. As chaves mudam com o `package-lock.json`, e o build do Angular por commit, com volta ao mais recente;
  - os scripts de seleção são código a manter (regras do que é global).
- **Na prática:**
  - **arquivo novo que vale para todas as telas** (estilo global, serviço do `core/`) entra nas regras de `GLOBAIS` do `testes-afetados.py`, se não estiver numa pasta que já está lá;
  - **teste no navegador novo** abre a tela pela rota escrita (`page.goto('/transacoes')`), para a seleção achar;
  - **ver o que rodaria aqui:** `python3 scripts/ci/testes-afetados.py --base origin/develop` (mostra os comandos);
  - **rodar aqui antes do push** (#286, 2026-10-21): `cd src/frontend && NAVEGADOR=chromium npm run test:afetados`. Ele usa a mesma decisão, soma o que ainda não foi commitado (`--local`) e roda o Jest, o build e o navegador (`--executar`). Chama o Node direto com o executável de cada ferramenta, porque no Windows o `npx.cmd` passa pelo `cmd.exe`, que lê o `<` de `<rootDir>` como redirecionamento;
  - **mexeu em CI:** tempo antes e depois no PR, por job (regra da #204);
  - **PR só de CI usa o título `ci:`** (#307, 2026-10-21): o `precisa-msi.py` não gera o `.msi` nele, mesmo mudando o `versao.yml` ou o próprio script. A salvaguarda: se o PR mexe no instalador, no gerenciador ou na inicialização da API, o `.msi` roda e o resumo avisa que o título não combina. O `.msi` continua rodando no push do `develop` e na tag.
