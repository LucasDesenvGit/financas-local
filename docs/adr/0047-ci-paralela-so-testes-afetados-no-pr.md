# 0047. CI em jobs paralelos e, no PR, só os testes afetados; .msi só quando o PR mexe nele

- **Status:** Aceita; a parte do .msi no PR foi substituída pela [0050](0050-msi-gerado-no-merge-e-nao-no-pr.md) (o .msi só no merge para o develop), e a escolha dos testes afetados saiu da CI em 2026-11-22 (#774, no fim)
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
- *(Saiu da CI em 2026-11-22, #774: ver a atualização no fim.)* **No PR, só os testes afetados** (`testes-afetados.py`): o navegador rodava os testes das telas (rotas do `app.routes.ts`) que importam o arquivo mudado, direta ou indiretamente (grafo de imports, filtro `--grep`); o Jest, `--findRelatedTests` mais os testes que leem o código como texto; tudo quando mudava arquivo global ou sem regra; nada quando o PR não mexia no front-end. No push do `develop`/`main`, sempre tudo.
- **Job "Go"** (#231, [ADR 0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)): roda em paralelo com os outros e entra no `build`.
  - Passos: `gofmt`, `go vet`, `go test -race`, código gerado em dia (`go generate` sem diferença), compilação para Windows e `govulncheck` duas vezes, no Linux e com `GOOS=windows`, porque ele só olha os arquivos do sistema em que roda (#449). O job do gerenciador em Go no Linux também roda o `govulncheck`, além do do Windows (#449).
  - No PR, só quando muda algo que o Go lê: `src/backend-go/`, `banco/`, o `2-criar-banco.sh`, o `Seguranca/` (a CSP que os testes do Go conferem) ou a própria CI. No push, sempre.
  - Cache do `setup-go`, com chave no `go.sum`: módulos e compilação, inclusive do `sqlc` e do `govulncheck`, que rodam por `go run` (ou `go install`, para a rodada com `GOOS=windows`) numa versão fixa.
- *(Substituído pela [ADR 0050](0050-msi-gerado-no-merge-e-nao-no-pr.md): o .msi não roda mais no PR, só no merge para o develop, na tag e à mão.)* **.msi só quando o PR precisa** (`scripts/ci/precisa-msi.py`): instalador, gerenciador, inicialização da API, o próprio workflow, ou o `Directory.Build.props` **além da linha `<Version>`**. No push do `develop` e na tag, sempre.
- **Caches:** módulos e compilação do Go (`setup-go`), `node_modules` (pula o `npm ci`), build do Angular (`angular.json`: `cli.cache.environment: "all"`, porque o padrão desliga na CI), Jest (`--cacheDirectory`), NuGet (também no job do .msi, que publica self-contained para win-x64), e o Chromium do Playwright, como antes.
  - **Chave de cache só com o que muda o conteúdo** (#526): nada que mude em todo PR, como o `Directory.Build.props` era na ADR 0039, quando subia em todo PR. Com ele na chave do NuGet (do .NET, que saiu na #86), cada PR criava um cache novo de ~700 MB, o repositório passou do limite de 10 GB do Actions, e o GitHub apagava os caches do Go, do Playwright e do CodeQL. A do NuGet leva os `.csproj` e a versão do SDK do .NET (que decide os pacotes do runtime do `.msi`).

## Consequências

- **Positivas:**
  - PR que não mexe no instalador deixa de esperar ~7 min do .msi;
  - o front-end testa em paralelo com a API, e o navegador em duas máquinas;
  - PR de uma tela rodava só os testes dela (ex.: Painel, 82 de 178 no navegador e 20 de 86 arquivos do Jest), até a #774.
- **Negativas:**
  - **um teste afetado podia escapar no PR** (até a #774): dependência que o grafo não vê (ex.: CSS de componente que muda outra tela por cascata, teste que abre a tela por um caminho sem a rota escrita). Mitigação: tudo no merge e arquivos globais disparam tudo;
  - **mais minutos de CI no total** (cada job prepara a própria máquina), em troca de menos tempo de relógio;
  - **cache grande demais:** o limite do Actions é de 10 GB por repositório, e o GitHub apaga os mais antigos sem avisar. O total fica em Actions › Caches (#526);
  - **cache que vale demais:** um `node_modules` ou build do Angular estragado no cache pode passar adiante. As chaves mudam com o `package-lock.json`, e o build do Angular por commit, com volta ao mais recente;
  - os scripts de seleção são código a manter (regras do que é global).
- **Na prática:**
  - **rodar aqui antes do push** (#286, 2026-10-21): `cd src/frontend && NAVEGADOR=chromium npm run test:afetados`, com o que ainda não foi commitado. Desde a #774, roda o Jest inteiro, o build e o navegador só nas telas afetadas (`scripts/dev/testes-afetados.py`): arquivo novo que vale para todas as telas entra em `GLOBAIS` dele, e teste novo no navegador abre a tela pela rota escrita (`page.goto('/transacoes')`), para a seleção achar;
  - **mexeu em CI:** tempo antes e depois no PR, por job (regra da #204);
  - **a API é ligada em todo PR de back-end** (#309, 2026-10-21): o job "API e banco" roda `scripts/ci/ligar-api.sh` (~10 s), que liga a API compilada num banco temporário, em HTTPS com certificado descartável, e espera `/api/auth/estado`. Pega o que só aparece ao iniciar (injeção de dependência, configuração, esquema). O `Program.cs` continua disparando o `.msi`, porque o que é só do Windows (serviço, DPAPI, certificado do instalador) só o teste do `.msi` cobre;
  - **PR só de CI usa o título `ci:`** (#307, 2026-10-21): o `precisa-msi.py` não gera o `.msi` nele, mesmo mudando o `versao.yml` ou o próprio script. A salvaguarda: se o PR mexe no instalador, no gerenciador ou na inicialização da API, o `.msi` roda e o resumo avisa que o título não combina. O `.msi` continua rodando no push do `develop` e na tag.

## Atualização de 2026-11-22: o front-end roda inteiro no PR (issue #774)

- **Medido** (18/11/2026, mediana de 25 PRs, com a escolha, contra 10 pushes, com tudo): Jest 1,2 contra 1,1 min; navegador 2,2/2,0 contra 2,3/2,2 min. A preparação da máquina (dependências, build, Chromium) pesa mais que os testes: o Jest inteiro leva ~8 s e o navegador inteiro ~40 s. O job mais lento do PR é o Go (Windows), com 4,8 min, então o front-end não decidia o tempo do PR. Em 400 execuções, os jobs do front-end falharam 0 vezes.
- **Decisão:** sempre que o front-end muda (`src/frontend/`, a CSP no `intermediarios.go` ou o `ci.yml`), Jest e navegador rodam inteiros, também no PR. O job "o que mudou" fica só com a saída `frontend`, um filtro de caminho como o do Go. O resto desta ADR continua.
- **Localmente**, o `npm run test:afetados` continua, só para o navegador: numa mudança de uma tela, ~16 s contra ~55 s com o navegador inteiro (22/11/2026; em 21/10, 45 s contra 2,7 min). O script foi para `scripts/dev/`.
- **Negativa:** PR de uma tela gasta mais minutos de CI (o navegador inteiro, nas duas partes), sem mudar o tempo de relógio.
