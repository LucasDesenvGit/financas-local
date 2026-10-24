# 0049. Back-end em Go, trocado aos poucos atrás do .NET

- **Status:** Aceita
- **Data:** 2026-10-21 (atualizada em 2026-10-23 com a convivência da #231 e o relógio de teste da #363)
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

### A convivência na prática (#231)

Como o .NET repassa ao Go, conferido de ponta a ponta pelo `scripts/ci/testar-go-atras-do-dotnet.sh` na CI:

- **Repasse com `HttpClient`, num endpoint.** Cada prefixo de `Go:Prefixos` vira um `app.Map("<prefixo>/{**resto}")` com ordem -1 (`RepasseGo`).
  - Por ser endpoint, a `FallbackPolicy` exige o login antes de chegar ao Go: sem login, 401 do próprio .NET, mesmo com o Go fora do ar ([autorização com política padrão](https://learn.microsoft.com/aspnet/core/security/authorization/secure-data#require-authenticated-users)).
  - A ordem -1 ganha dos controllers e do `MapFallback` de `/api` ([`WithOrder`](https://learn.microsoft.com/dotnet/api/microsoft.aspnetcore.builder.routingendpointconventionbuilderextensions.withorder)): um prefixo ligado leva o caminho, mesmo que um controller tenha a mesma rota.
  - **CSRF conferido no repasse.** O `AutoValidateAntiforgeryTokenAttribute` é um filtro do MVC e não vale para endpoint fora dele ([documentação](https://learn.microsoft.com/aspnet/core/security/anti-request-forgery#antiforgery-with-minimal-apis)). O repasse chama `IAntiforgery.ValidateRequestAsync` com a mesma regra do filtro: GET, HEAD, OPTIONS e TRACE não conferem; o resto sem token recebe 400 sem corpo.
  - Vão ao Go o método, o caminho, a query, o corpo e os cabeçalhos, menos os de conexão ([RFC 9110, 7.6.1](https://www.rfc-editor.org/rfc/rfc9110#section-7.6.1)), o cookie e o token CSRF: **o Go nunca vê o cookie de sessão.** Status, cabeçalhos e corpo voltam como o Go mandou, e o .NET põe os cabeçalhos de segurança por cima, como em toda resposta.
  - Go fora do ar (subindo, reiniciando, sem executável): 503 com mensagem.
- **Alternativa descartada: YARP** ([direct forwarding](https://learn.microsoft.com/aspnet/core/fundamentals/servers/yarp/direct-forwarding), pacote `Yarp.ReverseProxy`). Ele trata streaming, gRPC e WebSockets e converte erros em 5xx, mas seria um pacote novo na API para um repasse que sai na inversão (#85), e copia todos os cabeçalhos por padrão (inclusive o cookie). O repasse com `HttpClient` tem cerca de 100 linhas, lê a resposta sem guardá-la inteira na memória (`ResponseHeadersRead`) e não precisa de nada que o YARP tem a mais. *Julgamento:* se um assunto precisar de WebSocket ou streaming longo, trocar pelo `IHttpForwarder` do YARP, no mesmo endpoint.
- **Confiança entre os dois:**
  - o Go escuta só em `127.0.0.1`, numa porta que o sistema escolhe na partida (porta 0), e escreve `{"porta":N}` na saída padrão;
  - a conversa é **HTTP, sem TLS**: não sai do computador, e quem lê o tráfego de `127.0.0.1` já é administrador (e lê o banco). *Julgamento*;
  - **segredo interno** de 32 bytes aleatórios, gerado a cada partida do Go e passado pela **entrada padrão** (não aparece na linha de comando nem nas variáveis do processo). O Go confere com [`crypto/subtle.ConstantTimeCompare`](https://pkg.go.dev/crypto/subtle#ConstantTimeCompare) e, sem ele, responde 401 e registra um aviso;
  - o Id de quem está logado vai em `X-Financas-Usuario`. Todo cabeçalho `X-Financas-*` que vier de fora é apagado pelo .NET, e o Go os tira da requisição depois de conferir.
- **Processo filho, não serviço separado** (`ProcessoGo`). Continua um serviço só no systemd, no Serviço do Windows e no `.msi`, sem mudar o `configurar.ps1`.
  - Sobe no `app.Run()`, depois do `EsquemaBanco` e das rotinas de partida. **O Go não aplica esquema** e recusa subir sem o arquivo do banco (não cria um banco vazio).
  - Caiu: as rotas dele respondem 503, e o .NET o inicia de novo, esperando 1 s, 2 s, 4 s… até 1 min.
  - O .NET parou: fecha a entrada padrão, o Go para com elegância ([`http.Server.Shutdown`](https://pkg.go.dev/net/http#Server.Shutdown)), e passados 10 s é encerrado à força.
  - **O .NET caiu, até morto à força:** o sistema fecha a entrada padrão do Go, que para sozinho. No Windows, o Go fica também num [Job Object](https://learn.microsoft.com/windows/win32/procthread/job-objects) com `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` ([limites](https://learn.microsoft.com/windows/win32/api/winnt/ns-winnt-jobobject_basic_limit_information)), que o encerra mesmo travado. No systemd, o `KillMode=control-group` padrão já para o grupo todo ([systemd.kill](https://www.freedesktop.org/software/systemd/man/latest/systemd.kill.html)). *Julgamento a validar:* o Job Object foi testado só pelo teste do `.msi` e da instalação na CI do Windows, não num Windows de uso real.
  - O mesmo executável (`cmd/financas`, com `-atras-do-dotnet`) é o da futura API: não há um auxiliar a reescrever.
- **Log do Go:** em JSON na saída de erro ([`slog.JSONHandler`](https://pkg.go.dev/log/slog#JSONHandler)). O .NET registra cada linha com o mesmo nível na categoria `Financas.Go`, ou `Financas.Go.<origem>` quando o Go diz a origem da tela de erros (`"origem":"API"`). Avisos e erros vão para Configurações › Erros ([0022](0022-erros-do-sistema-gravados-no-banco.md)); a queda do Go vai como erro, com as últimas linhas da saída dele.
- **Banco em WAL** ([sqlite.org/wal.html](https://www.sqlite.org/wal.html)): o `EsquemaBanco` passa o arquivo para WAL a cada partida (fica gravado no arquivo; sem efeito se já está). Quem lê não espera quem grava, e dois que gravam esperam um ao outro:
  - no Go, `busy_timeout` de 30 s e `BEGIN IMMEDIATE` (#74);
  - no .NET, o Microsoft.Data.Sqlite já tenta de novo enquanto o banco estiver ocupado, até o tempo do comando (30 s), e começa as transações com `BEGIN IMMEDIATE` ([documentação](https://learn.microsoft.com/dotnet/standard/data/sqlite/database-errors#locking-retries-and-timeouts), [código](https://github.com/dotnet/efcore/blob/main/src/Microsoft.Data.Sqlite.Core/SqliteTransaction.cs)). Nada mudou do lado do .NET;
  - as cópias continuam certas: `VACUUM INTO` (a do `EsquemaBanco` e a do gerenciador) e `sqlite3 .backup` levam o que está no `-wal` (conferido), e o `configurar.ps1` já copia o `-wal` e o `-shm`. **Restaurar** uma cópia agora pede apagar o `-wal` e o `-shm` que sobraram ([sqlite.org/howtocorrupt.html, 1.4](https://www.sqlite.org/howtocorrupt.html)), com o serviço parado (dito no `4-backup.sh`).
- **Configuração** (`appsettings.json`, seção `Go`), lida na partida: mudar vale ao reiniciar o serviço, sem reinstalar.
  - `Go:Prefixos`: **uma entrada por assunto** (#363), com os prefixos dele separados por `;` (`"cambio": "/api/cambio;/api/cripto"`); entrada vazia é assunto desligado. O `appsettings.json` já traz as entradas dos assuntos do épico, vazias, com uma linha em branco entre elas: dois PRs em paralelo mexem cada um na sua linha, sem conflito no merge (o merge acusa conflito em linhas vizinhas, conferido). Só `/api/<algo>`, nunca `/api/auth` nem `/api/erros`.
    - Um `appsettings` de cima desliga um assunto sozinho (`"cambio": ""`). O texto com `;` continua valendo e ganha das entradas: `"Prefixos": ""` (ou `Go__Prefixos=""`) desliga todos.
    - **Lista JSON, não:** no .NET, uma lista vazia num `appsettings` de cima não apaga a de baixo. Na #231 era um texto só, e cada assunto mexeria na mesma linha.
  - `Go:Tarefas`: tarefas em segundo plano que passaram para o Go e não rodam no .NET (`Sincronizacao`, `Cambio`, `Cripto`). O gravador de erros fica no .NET até a #298.
  - `Go:Executavel`: o executável (padrão: `financas-go`, ao lado da API).
  - `Go:InicioDoRelogio`: **só para teste**, vazio por padrão (#363). Com um instante RFC 3339 com fuso, o .NET sobe o Go com `-inicio-do-relogio` e o relógio do Go começa ali (abaixo, em "Testes de contrato"). O .NET valida o instante e passa o valor que ele mesmo formata, nunca o texto da configuração; valor inválido não deixa o serviço subir.
  - Sem prefixo e sem tarefa, o Go nem é iniciado.
- **Instalação mínima** (o resto é da #84), atualizando a [0011](0011-ferramentas-de-instalacao-portateis-sem-python.md):
  - Go portátil em `ferramentas/go`, da linha do `go.mod` (`go 1.27.3` → a 1.27.x mais nova), pelo índice de `go.dev/dl` e o arquivo de `dl.google.com`, com o **SHA-256 do próprio índice conferido**. Reaproveita um `go` do PATH da mesma linha e não mais velho;
  - compilado pelo `3-publicar-atualizar.sh`, pelo `instalar-windows.ps1` e pelo `gerar-msi.ps1` (com o `setup-go` no `versao.yml`), sem cgo (`CGO_ENABLED=0`), com a versão do `Directory.Build.props` pelo `-ldflags`, e publicado ao lado da API;
  - `GOTOOLCHAIN=local` (nunca baixar outra versão do Go sozinho, [toolchains](https://go.dev/doc/toolchain)) e o cache de módulos e de compilação em `ferramentas/`;
  - **sem `go mod vendor`.** Medido: a pasta `vendor` teria 137 MB (128 MB só do `modernc.org`, que traz o SQLite de todas as plataformas), no repositório. Os módulos vêm de `proxy.golang.org`, conferidos pelo `go.sum` do repositório, que guarda o hash de cada um ([go.sum](https://go.dev/ref/mod#go-sum-files)): a mesma garantia de integridade do SHA-256 conferido, sem o peso no `git clone`. Custo: mais um domínio na instalação, e compilar precisa de internet (como o NuGet e o npm já precisam).

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
| Certificado `.pfx` | [`software.sslmate.com/src/go-pkcs12`](https://pkg.go.dev/software.sslmate.com/src/go-pkcs12) (#73) | A biblioteca padrão não lê PKCS#12. Os geradores de hoje (`dotnet dev-certs` e `Export-PfxCertificate`) usam o formato antigo (3DES), que o `golang.org/x/crypto/pkcs12` também lê. Mas ele [está congelado](https://pkg.go.dev/golang.org/x/crypto/pkcs12) e não lê o `.pfx` cifrado com AES (testado: falha no SHA-256), que o OpenSSL 3 gera por padrão. O `go-pkcs12`, um fork mantido, lê os dois. Conferido: o Go leu o `.pfx` do `dotnet dev-certs` e serviu o mesmo certificado do .NET. *Julgamento a revisar:* trocar pelo `x/crypto/pkcs12` tira uma dependência de terceiro, ao custo de quebrar com um `.pfx` AES. |

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
  - **Domínios novos na instalação** (#231): `go.dev` e `dl.google.com` (o Go, com o SHA-256 conferido) e `proxy.golang.org` (os módulos, conferidos pelo `go.sum`). A [0011](0011-ferramentas-de-instalacao-portateis-sem-python.md) foi atualizada.
  - **Instalação pelo código maior durante a transição** (medido no Linux x64, #231): o Go portátil ocupa 276 MB em `ferramentas/go` (mais 68 MB do arquivo baixado), o cache de módulos 306 MB (46 MB baixados) e o de compilação 155 MB. A primeira compilação do Go leva cerca de 15 s; o executável tem 12 MB. O `.msi` só ganha o executável.
  - **O Go é processo filho:** se o .NET não subir, o Go também não sobe; e um Go travado só é encerrado à força pelo .NET (ou, no Windows, pelo Job Object).
- **Na prática:**
  - **Código Go em `src/backend-go/`** ([Organizing a Go module](https://go.dev/doc/modules/layout)):
    - `cmd/financas/main.go`;
    - `internal/` com um pacote por assunto e os pacotes de apoio (`config`, `servidor`, `banco`, `relogio`). Cada assunto exporta `Registrar(mux, <o que usa>)`, e o `internal/api` tem uma linha por assunto (#363).
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
    - o `Abrir` liga `foreign_keys` como o EF Core, espera até 30 s com o banco ocupado e começa as transações com `BEGIN IMMEDIATE`. Não muda o `journal_mode`: quem passa o banco para WAL é quem aplica o esquema, o `EsquemaBanco` do .NET ou o `AplicarEsquema` do Go (#231, #297);
    - o `sqlc` roda pelo `go run` numa versão fixa, sem entrar no `go.mod`, e precisa de cgo só para quem gera. Ele lê um esquema reescrito pelo próprio SQLite (o parser dele não entende `TEXT NULL` e passa para minúsculas os nomes do `CREATE TABLE`). Nas consultas, os nomes vão entre aspas duplas e não pode haver acento.
  - **Servidor (#73):**
    - o Go lê os mesmos `appsettings*.json` e o `configuracao.json` do `.msi`, então os scripts não mudam;
    - escuta só em endereço local, e recusa outro;
    - os cabeçalhos, o cache, o `ETag`, o 401 e o 404 sem corpo e o 400 de host foram conferidos contra o .NET rodando, rota por rota (`internal/servidor/igual_dotnet_test.go`);
    - **o `wwwroot` é lido do disco**, ao lado do executável, como no .NET: embutir com `//go:embed` obrigaria compilar o Angular antes do Go e muda a publicação, e pode ser revisto na #84;
    - **a CSP continua no `Seguranca.cs`:** um teste do Go lê o C# e confere que é a mesma, até o .NET sair (#86).
  - **Testes de contrato (#72; detalhes no `src/backend-go/testes/contrato/README.md`):**
    - os casos ficam em `casos/*.json`, um arquivo por assunto, rodados em ordem; o que o .NET respondeu fica em `esperado/*.json` (*golden files*), com o status, o JSON e as linhas que cada caso mudou em todas as tabelas, com o texto cru de cada coluna. Um caso pode guardar um valor da resposta para os seguintes (`capturar`);
    - o JSON é comparado sem passar por `float64` (`10.50` ≠ `10.5`). Só é normalizado o que muda a cada execução (hora do relógio, GUID novo, `traceId`, versão, sal da senha, pasta temporária); dinheiro, datas de negócio e ids nunca;
    - o .NET roda sem mudar o código: sem rede (`unshare -rn`, porque o câmbio e a cripto acessam a internet sozinhos), com o relógio começando em 2026-10-15 01:00 UTC pela libfaketime (o modo "começa em", sem mexer no relógio monotônico; congelado, travaria os temporizadores), em pt-BR e no fuso de Brasília. O banco fictício é inventado (`dados/base.sql`);
    - a libfaketime não age sobre o Go: ele lê o relógio direto do kernel, pelo vDSO, sem a libc (conferido: sob a libfaketime, o `date` vê o instante falso e o Go, a hora real). Por isso o Go tem uma **opção de teste**, `-inicio-do-relogio` (#363): o relógio (`relogio.APartirDe`) começa no instante dado e anda dali, no relógio monotônico, como o `FAKETIME="@..."`. Sem a opção, o relógio do sistema. O contrato liga `Go:InicioDoRelogio`, e o .NET passa ao Go esse instante mais o tempo desde a partida dele: o Go fica no mesmo relógio do .NET, e continua nele se reiniciar;
    - o contrato roda **contra o Go atrás do .NET** (#363): `PREFIXOS_GO` no `rodar-dotnet.sh` repassa ao Go os prefixos de um assunto, e os mesmos esperados, gravados do .NET, têm de passar. Gravar esperados com o Go respondendo é recusado, menos nas rotas que só existem no Go (`/api/go`). Cada assunto tem um número de arquivo de casos reservado e liga a rodada contra o Go na linha dele do `assuntos-no-go.txt`, que a CI lê (`rodar-dotnet.sh --go`), sem mexer no `ci.yml` (tabela e passos no README do contrato);
    - a CI roda o contrato contra o .NET, com o Go compilado e atrás dele (job "contrato (.NET)"), em todo PR que mexe no back-end;
    - cada etapa grava os casos do próprio assunto, com `--atualizar` contra o .NET, no PR dela.
  - **Os outros testes do Go** (unitário em tabela, integração com SQLite real, HTTP com `httptest`) e o "pronto quando" de cada etapa estão na [0051](0051-testes-do-back-end-em-go-cada-regra-com-teste.md) (#368): o contrato continua obrigatório, mas não basta sozinho.
  - **Versão:** entra no executável por `-ldflags -X`, lida do `Directory.Build.props` (a [0039](0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md) continua valendo).
  - **CSP:** os testes no navegador ([0041](0041-testes-no-navegador-com-playwright.md)) e o `testes-afetados.py` ([0047](0047-ci-paralela-so-testes-afetados-no-pr.md)) leem a CSP do `Seguranca.cs`. Ela ganha uma fonte única antes de o Go servir o front-end (#73).
  - **Esquema do banco** ([0003](0003-esquema-por-scripts-sql-idempotentes.md)): continua nos scripts de `banco/` mais as linhas `adicionar_coluna_se_ausente`, aplicado pelo .NET até a inversão e depois pelo Go (#297).
    - O porte é o `banco.AplicarEsquema`, com a **mesma assinatura** no `PRAGMA user_version`: trocar entre o .NET e o Go não refaz a cópia do banco. Conferido contra o `EsquemaBanco.cs` por uma sonda que compila o mesmo arquivo (`internal/banco/testdata/sonda-esquema`) e contra a API rodando, nos dois sentidos.
    - **Até a #85, o Go não aplica o esquema ao iniciar:** só com a opção `-aplicar-esquema`, desligada por padrão.
    - **Os scripts vão dentro do executável por cópia:** o `//go:embed` não alcança arquivos fora do módulo, e o `banco/` fica fora de `src/backend-go`. O `go generate` copia os três arquivos para `internal/banco/esquema/embutido/`, e o teste e a CI ("go generate em dia") falham se a cópia ficar diferente. *Alternativas descartadas:* pôr o `go.mod` na raiz do repositório (mexe em todo o módulo e no `go` da CI) e ler os scripts do disco ao lado do executável (a instalação teria de levá-los, e o `.msi` não leva hoje).
    - **Diferença aceita:** o nome da cópia leva a hora de Brasília; o .NET usa a hora do computador.
  - **Erros** ([0022](0022-erros-do-sistema-gravados-no-banco.md)): registrados pelo .NET até a inversão, e depois por um handler do `slog` (#298).
  - **Gerenciador:** a [0028](0028-gerenciador-do-windows-e-atualizacao-pelo-github.md), a [0033](0033-gerenciador-no-linux-com-avalonia.md) e a [0040](0040-atualizar-pelo-develop-so-na-instalacao-de-desenvolvimento.md) mudam quando a #276 decidir como ele vai para o Go.
  - **Todo PR do épico** traz a seção "🎓 Aula rápida". A skill `migrar-para-go` e o agente `dev-go` do Claude Code seguem esta ADR.
