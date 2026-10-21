# 0049. Back-end em Go, trocado aos poucos atrás do .NET

- **Status:** Aceita
- **Data:** 2026-10-21
- **Relacionadas:** substitui em parte, ao fim da migração, a [0002](0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md) (Kestrel), a [0005](0005-segredos-criptografados-com-data-protection.md) (Data Protection), a [0011](0011-ferramentas-de-instalacao-portateis-sem-python.md) e a [0012](0012-instalador-do-windows-em-powershell.md) (SDK do .NET); afeta a [0003](0003-esquema-por-scripts-sql-idempotentes.md), a [0022](0022-erros-do-sistema-gravados-no-banco.md), a [0039](0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md), a [0041](0041-testes-no-navegador-com-playwright.md) e a [0047](0047-ci-paralela-so-testes-afetados-no-pr.md); épico #70, issues #71 e #231

## Contexto

O time decidiu trocar o back-end de C#/.NET 10 por **Go**, e depois o gerenciador (#276), até não sobrar .NET no projeto. Os objetivos são:
- uma instalação mais leve: um executável por programa, sem runtime, com compilação cruzada para Windows e Linux;
- aprender Go no caminho: cada PR do épico traz uma "🎓 Aula rápida".

A linha de base, medida em 21/10/2026 na versão 2.23 instalada pelo `.msi`, é **157 MB** de pasta e **~225 MB** de memória da API. Os números com Go são estimativas até a medição na #277 e na #86.

O que a migração tem de respeitar, conferido no código em 21/10/2026:

- **Tamanho:** 128 rotas em `Controllers/`, 4 tarefas em segundo plano (`Program.cs:135-142`) e cerca de 15,3 mil linhas de C#. O back-end **não tem teste automatizado**.
- **O C# continua mudando:** ganhou cerca de 40% de rotas desde que o épico foi escrito.
- **Banco:** o EF Core grava no SQLite em formatos próprios: decimal como `TEXT` (`-5000.0`), data e hora como `TEXT` com 7 casas de fração, GUID em maiúsculas.
- **Login:** o cookie de sessão e o token CSRF são cifrados pelo Data Protection do ASP.NET, num formato que o Go não lê. O mesmo vale para os segredos da tabela `Configuracao` (Client Secret da Pluggy, token da brapi).
- **Partida da API:** ela aplica o esquema do banco (`EsquemaBanco`, `Program.cs:195`) e roda rotinas de acerto que gravam no banco (`Program.cs:205-220`).

## Alternativas consideradas

- **Manter o .NET.** Descartada: a decisão do time é ter tudo em Go.
- **Node + TypeScript.** Reaproveitaria o Node que o front-end já usa para compilar. Contra: leva o runtime do Node para a máquina de quem usa, e não entrega "um executável, sem runtime".
- **Python.** Contra: também precisa de runtime, e a [0011](0011-ferramentas-de-instalacao-portateis-sem-python.md) tirou o Python dos scripts de propósito.
- **Go, com troca única.** Era o plano original do #70: o Go cresce ao lado e substitui o .NET de uma vez, num passo com volta automática. Contra: concentra todo o risco num passo só, num back-end sem testes, e o código novo só entra em uso depois de tudo pronto.
- **Go aos poucos, com o Go lendo o cookie do .NET.** Contra: o formato do Data Protection é exclusivo do .NET. Dividir a autenticação entre os dois exigiria enfraquecê-la durante a transição.
- **Go aos poucos, com o .NET na frente.** O .NET continua cuidando do login e repassa ao Go as rotas já migradas. O Go nunca precisa ler o cookie. **Escolhida.**

## Decisão

Reescrevemos o back-end em **Go**, num módulo em `src/backend-go/`, e o trocamos **aos poucos, um assunto de cada vez**, no padrão *strangler fig* ([Martin Fowler](https://martinfowler.com/bliki/StranglerFigApplication.html), [Microsoft](https://learn.microsoft.com/en-us/azure/architecture/patterns/strangler-fig)). O código legado continua funcionando enquanto o Go o substitui.

### O .NET na frente até a inversão

- **O .NET continua cuidando**, até o fim, de login, sessão, cookie, CSRF, limite de tentativas, segredos, esquema do banco, rotinas de partida e registro de erros.
- **Repasse por prefixo** (#231): o .NET repassa ao Go os prefixos de rota de uma **lista na configuração**, com a pessoa autenticada e o CSRF já conferido. Prefixo fora da lista continua no .NET.
- **Voltar um assunto para o .NET é tirar o prefixo da lista.** Não precisa reinstalar nem restaurar o banco.
- **Na inversão, no fim** (#85), o Go passa a ficar na frente, com a autenticação (#75), os segredos (#76), os erros (#298) e o esquema do banco (#297), e o repasse sai.

### Cada assunto entra em uso em três passos

1. **Contrato e código.** Os casos de contrato do assunto (#72) são gravados do .NET **antes** do código Go: mesmo status, mesmo JSON e o banco no mesmo estado. O código Go entra com o prefixo ainda desligado.
2. **Prefixo ligado.** A tarefa em segundo plano do assunto, se houver, é **desligada no .NET no mesmo passo**, para não rodar duas vezes.
3. **C# apagado.** Depois de **uma versão publicada sem precisar voltar**, um PR à parte apaga o C# do assunto, só o que **nenhum outro código C# ainda usa**.
   - O compartilhado (entidades, `FinancasDbContext`, `Relogio`, serviços chamados por outros assuntos) fica até o último C# que o usa sair.
   - O que sobrar sai na #86.

### Segredos: o esquema novo é feito cedo, mas só é usado na troca

- **O esquema novo é escrito em Go** (AES-256-GCM, valor com prefixo `v2:`, chave em `dados/chaves` protegida por DPAPI no Windows e por permissão de arquivo no Linux; #76), mas **fica desligado**.
- **Até a troca, o .NET continua sendo a fonte dos segredos.** Não há canal interno pelo qual o Go peça um segredo ao .NET.
- **Assunto que depende de segredo só liga o prefixo junto com a troca dos segredos:** a cotação pela brapi, na #82, e a Pluggy, na #83.
- **A troca dos segredos** acontece quando o núcleo estiver completo, ou na inversão. Nesse passo, quem usa informa o Client Secret da Pluggy e o token da brapi de novo, uma vez.

### Repetição e tabela com dois escritores na transição

- **Regra de negócio repetida é aceita até o assunto que a usa migrar.** Exemplo: a categorização em C#, chamada pela sincronização, e em Go, chamada pelas transações. Valem duas condições:
  - o **mesmo caso de contrato roda contra os dois lados**;
  - mudou um lado, **o mesmo PR muda o outro**.
- **Uma tabela pode ter dois escritores** (.NET e Go) durante a transição. Isso só é seguro porque:
  - o Go grava no **formato do EF Core** (#74);
  - o SQLite fica em **WAL**, com `busy_timeout` nos dois lados;
  - as transações são curtas.

### Bibliotecas

| Necessidade | Biblioteca | Por quê |
|---|---|---|
| HTTP e rotas | `net/http` da biblioteca padrão | Desde o Go 1.22 o `ServeMux` entende método e parâmetro no padrão (`GET /api/contas/{id}`) ([blog do Go](https://go.dev/blog/routing-enhancements)). Sem framework, menos dependência. |
| SQLite | `modernc.org/sqlite` | Porte do SQLite para Go sem cgo ([pkg.go.dev](https://pkg.go.dev/modernc.org/sqlite)): compila para Windows sem compilador C. |
| Consultas | [`sqlc`](https://docs.sqlc.dev/en/latest/tutorials/getting-started-sqlite.html) + `database/sql`, com `?` nos filtros dinâmicos | Gera código Go tipado a partir do SQL, com motor `sqlite`. O esquema vira a fonte única, e acaba a sincronia manual com o `FinancasDbContext`. |
| Dinheiro | [`github.com/shopspring/decimal`](https://pkg.go.dev/github.com/shopspring/decimal) | Go não tem decimal nativo, e `float64` erra centavos. Tem `RoundBank`, o arredondamento padrão do `Math.Round` do C#. |
| Serviço do Windows | [`golang.org/x/sys/windows/svc`](https://pkg.go.dev/golang.org/x/sys/windows/svc) | Mantido pelo time do Go. |
| Logs | `log/slog` da biblioteca padrão | Log estruturado sem dependência ([blog do Go](https://go.dev/blog/slog)). |

Dependência fora desta lista precisa de atualização desta ADR, ou de uma ADR nova.

## Consequências

- **Positivas:**
  - o legado continua funcionando, e cada passo é pequeno, em uso real e reversível pela configuração;
  - o código novo entra em uso aos poucos, em vez de esperar o fim do épico;
  - no fim, um executável por programa, sem runtime;
  - o `sqlc` torna o esquema do banco a fonte única das consultas;
  - o back-end ganha testes automatizados (os de contrato), que hoje não existem.
- **Negativas:**
  - **Reescrita grande:** 128 rotas, 4 tarefas e cerca de 15,3 mil linhas, numa linguagem que o time está aprendendo.
  - **Dois runtimes juntos por mais tempo:** a instalação fica **maior** durante a transição e só encolhe no fim. Quem mexe no back-end precisa das duas linguagens.
  - **Um salto a mais** em cada rota repassada (local, pequeno).
  - **Regra repetida pode divergir** se alguém esquecer o outro lado. O contrato rodando contra os dois é a defesa.
  - **Dois escritores na mesma tabela** exigem o formato do EF Core sem desvio. Um erro de formato no Go aparece como erro de leitura no .NET.
  - **Assuntos com segredo** (#82, #83) ficam no .NET até a troca dos segredos, mesmo com o código Go pronto.
  - **Na inversão, todos são deslogados uma vez** (o formato do cookie muda) e **os segredos são informados de novo uma vez**. Se for preciso voltar ao .NET depois disso, os segredos são informados de novo, porque o .NET não lê o `v2:`.
  - **O C# continua mudando:** mudou no C# um assunto já portado, o mesmo PR muda o Go e o contrato.
  - **Pegadinhas da linguagem:**
    - o RE2 do Go não aceita lookbehind nem lookahead, e o `\b` dele só conhece ASCII (`ExtratoCarteiraService.cs:43`, `BancoService.cs:55`);
    - o `shopspring/decimal` escreve o número **entre aspas** no JSON, a menos que `MarshalJSONWithoutQuotes` seja ligado ([pkg.go.dev](https://pkg.go.dev/github.com/shopspring/decimal)). O tipo de dinheiro do projeto (#74) define o próprio `MarshalJSON`;
    - o suporte do `sqlc` a SQLite é menos maduro que o de PostgreSQL: consulta que ele não gerar vai para `database/sql` com `?`.
  - **Domínio novo na instalação:** o download do Go (`go.dev`, `dl.google.com`) entra nos scripts na #231, com o SHA-256 conferido, e a [0011](0011-ferramentas-de-instalacao-portateis-sem-python.md) é atualizada nesse PR.
- **Na prática:**
  - **Código Go em `src/backend-go/`** ([Organizing a Go module](https://go.dev/doc/modules/layout)):
    - `cmd/financas/main.go`;
    - `internal/` com um pacote por assunto e os pacotes de apoio (`config`, `servidor`, `banco`, `relogio`).
  - **Convenções do código Go** (também no `CLAUDE.md`):
    - nomes em português;
    - erro como valor, embrulhado com `%w`;
    - `context.Context` como primeiro parâmetro de quem faz I/O;
    - dependências pelo construtor, sem container;
    - relógio injetável no fuso de Brasília;
    - SQL só pelo `sqlc` ou com `?`, nunca com texto do usuário concatenado;
    - dinheiro só em decimal, e nada de `SUM`, `ORDER BY` ou `<`/`>` no SQL em coluna de dinheiro;
    - `gofmt`, `go vet` e `go test -race` limpos.
  - **Banco (#74; detalhes no `src/backend-go/internal/banco/README.md`):**
    - os tipos `Decimal`, `Data`, `DataHora` e `Guid` (pacote `internal/tipos`) gravam e leem no formato do EF Core. O formato foi confirmado por uma sonda que grava pelo mesmo EF Core da API (`internal/tipos/testdata/sonda-efcore`), e o resultado é o gabarito dos testes;
    - o `Abrir` liga `foreign_keys` como o EF Core, espera até 30 s com o banco ocupado e começa as transações com `BEGIN IMMEDIATE`. Não muda o `journal_mode`;
    - o `sqlc` roda pelo `go run` numa versão fixa, sem entrar no `go.mod`, e precisa de cgo só para quem gera. Ele lê um esquema reescrito pelo próprio SQLite (o parser dele não entende `TEXT NULL` e passa para minúsculas os nomes do `CREATE TABLE`). Nas consultas, os nomes vão entre aspas duplas e não pode haver acento.
  - **Versão:** entra no executável por `-ldflags -X`, lida do `Directory.Build.props` (a [0039](0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md) continua valendo).
  - **CSP:** os testes no navegador ([0041](0041-testes-no-navegador-com-playwright.md)) e o `testes-afetados.py` ([0047](0047-ci-paralela-so-testes-afetados-no-pr.md)) leem a CSP do `Seguranca.cs`. Ela ganha uma fonte única antes de o Go servir o front-end (#73).
  - **Esquema do banco** ([0003](0003-esquema-por-scripts-sql-idempotentes.md)): continua nos scripts de `banco/` mais as linhas `adicionar_coluna_se_ausente`, aplicado pelo .NET até a inversão e depois pelo Go (#297). Erros ([0022](0022-erros-do-sistema-gravados-no-banco.md)): registrados pelo .NET até a inversão, e depois por um handler do `slog` (#298).
  - **Gerenciador:** a [0028](0028-gerenciador-do-windows-e-atualizacao-pelo-github.md), a [0033](0033-gerenciador-no-linux-com-avalonia.md) e a [0040](0040-atualizar-pelo-develop-so-na-instalacao-de-desenvolvimento.md) mudam quando a #276 decidir como ele vai para o Go.
  - **Todo PR do épico** traz a seção "🎓 Aula rápida". A skill `migrar-para-go` e o agente `dev-go` do Claude Code seguem esta ADR.
