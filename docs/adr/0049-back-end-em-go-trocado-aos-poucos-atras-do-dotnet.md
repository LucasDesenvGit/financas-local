# 0049. Back-end em Go, trocado aos poucos atrás do .NET

- **Status:** Aceita (concluída na #86, em 2026-11-03: o .NET saiu, e o site é o Go sozinho)
- **Data:** 2026-10-21 (atualizada em 2026-10-23 com a convivência da #231 e o relógio de teste da #363; encurtada em 2026-11-16, na #729, sem mudar a decisão)
- **Relacionadas:** substitui em parte, ao fim da migração, a [0002](0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md) (Kestrel), a [0005](0005-segredos-criptografados-com-data-protection.md) (Data Protection), a [0011](0011-ferramentas-de-instalacao-portateis-sem-python.md) e a [0012](0012-instalador-do-windows-em-powershell.md) (SDK do .NET); afeta a [0003](0003-esquema-por-scripts-sql-idempotentes.md), a [0022](0022-erros-do-sistema-gravados-no-banco.md), a [0039](0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md), a [0041](0041-testes-no-navegador-com-playwright.md) e a [0047](0047-ci-paralela-so-testes-afetados-no-pr.md); épico #70, issues #71 e #231

## Contexto

O time decidiu trocar o back-end de C#/.NET 10 por **Go**, e depois o gerenciador (#276), até não sobrar .NET no projeto. Os objetivos: uma instalação mais leve (um executável por programa, sem runtime, com compilação cruzada para Windows e Linux) e aprender Go no caminho. A linha de base, medida em 21/10/2026 na versão 2.23, era **157 MB** de pasta e **~225 MB** de memória da API.

O que a migração tinha de respeitar, conferido no código em 21/10/2026:

- **Tamanho:** 128 rotas, 4 tarefas em segundo plano e cerca de 15,3 mil linhas de C#, **sem nenhum teste automatizado**. O C# continuava mudando.
- **Banco:** o EF Core grava no SQLite em formatos próprios: decimal como `TEXT` (`-5000.0`), data e hora como `TEXT` com 7 casas de fração, GUID em maiúsculas.
- **Login e segredos:** o cookie de sessão, o token CSRF e os segredos da `Configuracao` eram cifrados pelo Data Protection do ASP.NET, que o Go não lê.
- **Partida:** a API aplicava o esquema do banco e rodava rotinas que acertam dados.

## Alternativas consideradas

- **Manter o .NET.** Descartada: a decisão do time é ter tudo em Go.
- **Node + TypeScript.** Reaproveitaria o Node do build do front-end. Contra: leva um runtime para a máquina de quem usa.
- **Python.** Contra: também precisa de runtime, e a [0011](0011-ferramentas-de-instalacao-portateis-sem-python.md) tirou o Python dos scripts de propósito.
- **Go, com troca única** (o plano original do #70). Contra: todo o risco num passo só, num back-end sem testes, e o código novo só em uso no fim.
- **Go aos poucos, com o Go lendo o cookie do .NET.** Contra: o formato do Data Protection é exclusivo do .NET, e dividir a autenticação a enfraqueceria na transição.
- **Go aos poucos, com o .NET na frente.** O .NET cuida do login e repassa ao Go as rotas já migradas; o Go nunca lê o cookie. **Escolhida.**

## Decisão

Reescrevemos o back-end em **Go** (hoje em `src/backend`; até a #627, `src/backend-go`) e o trocamos **aos poucos, um assunto de cada vez**, no padrão *strangler fig* ([Martin Fowler](https://martinfowler.com/bliki/StranglerFigApplication.html), [Microsoft](https://learn.microsoft.com/en-us/azure/architecture/patterns/strangler-fig)).

### Como foi a troca

- **O .NET na frente até a inversão** (#231), com o login, os segredos, o esquema, as rotinas de partida e os erros. Ele repassava ao Go os prefixos de rota de uma lista na configuração, com a pessoa autenticada e o CSRF conferidos. Voltar um assunto era tirar o prefixo da lista.
- **O Go era processo filho do .NET**, só em `127.0.0.1`, com um segredo interno pela entrada padrão, e **nunca via o cookie de sessão**. O repasse era um endpoint com `HttpClient`; o YARP ficou de fora (pacote novo para algo temporário, e copia o cookie por padrão).
- **Cada assunto em três passos:** os casos de contrato gravados do .NET antes do código Go; o prefixo ligado, com a tarefa do assunto desligada no .NET no mesmo passo; o C# apagado depois de uma versão publicada sem precisar voltar.
- **Aceito só na transição:** regra repetida nos dois lados (o mesmo contrato contra os dois, o mesmo PR mudando os dois) e tabela com dois escritores (o Go no formato do EF Core, WAL com `busy_timeout` e transações curtas). Assunto com segredo só ligou junto com a troca dos segredos.
- **Na inversão** (#85), o Go passou para a frente com a autenticação, os segredos, os erros e o esquema, e o C# saiu na #86.

**Para ver como era:** a tag `v-ultimo-dotnet`, o último `develop` com o .NET (3.0.2), guarda o C#, a convivência (`Go:Prefixos`, `Go:Tarefas`, o processo filho) e a skill da migração, que saiu na #727. O passo a passo que esta ADR guardava até a #729 está no CHANGELOG das versões 2.x e 3.0 e no histórico do git (`git show 135d15e5:docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md`).

### O que continua valendo

- **O formato do banco do EF Core** (decimal, data e GUID como texto) e **o banco em WAL**, com `busy_timeout` de 30 s, `BEGIN IMMEDIATE` e transação curta: [README do banco](../../src/backend/internal/plataforma/banco/README.md). Restaurar uma cópia apaga o `-wal` e o `-shm` ([guia de operação](../OPERACAO.md)).
- **O JSON de antes**, que o contrato confere (as mensagens de validação saem em português desde a #710); desde a #86, os esperados são a regressão do Go ([0051](0051-testes-do-back-end-em-go-cada-regra-com-teste.md)).
- **Segredos** (#76, no lugar da [0005](0005-segredos-criptografados-com-data-protection.md)): AES-256-GCM, valor `v2:` com nonce novo a cada gravação e o nome da chave da `Configuracao` como dado autenticado. A chave fica em `dados/chaves/segredos.chave` (DPAPI no Windows, `0600` no Linux); chave que não se lê é erro, nunca uma chave nova por cima.
- **Sessão** (#75): cookie assinado com HMAC-SHA256 pela chave `dados/chaves/sessao-go.chave`, e CSRF *double-submit* assinado e ligado ao usuário. A senha continua no hash v3 do ASP.NET Identity.
- **Esquema** ([0003](0003-esquema-por-scripts-sql-idempotentes.md)): o Go o aplica ao iniciar, com a assinatura do .NET no `PRAGMA user_version`. Os scripts vão no executável por uma cópia do `go generate`, porque o `//go:embed` não sai do módulo.
- **Rotinas de partida** (#444): uma transação por rotina; a que falha vira aviso, e a partida segue (o .NET parava na primeira; diferença aceita em 01/11/2026).
- **Erros** por um handler do `slog` ([0022](0022-erros-do-sistema-gravados-no-banco.md)); **versão** pelo `-ldflags`, calculada pelo `cmd/versao` a partir da base do `Directory.Build.props` ([0070](0070-versao-calculada-no-build-e-changelog-um-arquivo-por-pr.md)); **instalação** com o Go portátil, `GOTOOLCHAIN=local`, sem cgo e sem `go mod vendor` ([0011](0011-ferramentas-de-instalacao-portateis-sem-python.md)).
- **Convenções do código Go:** as regras de ouro estão em [`estilo.md`](../../.claude/skills/go-no-projeto/referencias/estilo.md), da skill `go-no-projeto` (o dinheiro, também no `CLAUDE.md`); as camadas, na [0056](0056-arquitetura-do-codigo-go-camadas-por-assunto-e-ddd-estrategico.md); a "🎓 Aula rápida" de todo PR com Go, no [README do Go](../../src/backend/README.md).

### Bibliotecas

| Necessidade | Biblioteca | Por quê |
|---|---|---|
| HTTP e rotas | `net/http` da biblioteca padrão | Desde o Go 1.22 o `ServeMux` entende método e parâmetro no padrão (`GET /api/contas/{id}`) ([blog do Go](https://go.dev/blog/routing-enhancements)). Sem framework, menos dependência. |
| SQLite | `modernc.org/sqlite` | Porte do SQLite para Go sem cgo ([pkg.go.dev](https://pkg.go.dev/modernc.org/sqlite)): compila para Windows sem compilador C. |
| Consultas | [`sqlc`](https://docs.sqlc.dev/en/latest/tutorials/getting-started-sqlite.html) + `database/sql`, com `?` nos filtros dinâmicos | Gera código Go tipado a partir do SQL, com motor `sqlite`. O esquema vira a fonte única, e acaba a sincronia manual com o `FinancasDbContext`. |
| Dinheiro | [`github.com/shopspring/decimal`](https://pkg.go.dev/github.com/shopspring/decimal) | Go não tem decimal nativo, e `float64` erra centavos. Tem `RoundBank`, o arredondamento padrão do `Math.Round` do C#. **Exceção (#82):** o fator da Selic, que não é dinheiro e que o C# acumula em `double`: o `internal/selic` usa `float64` só dentro da série, para dar o mesmo número, e devolve `tipos.Decimal`. |
| Serviço do Windows | [`golang.org/x/sys/windows/svc`](https://pkg.go.dev/golang.org/x/sys/windows/svc) e [`svc/eventlog`](https://pkg.go.dev/golang.org/x/sys/windows/svc/eventlog) | Mantidos pelo time do Go, no mesmo módulo. |
| Texto sem acento (Unicode) | [`golang.org/x/text/unicode/norm`](https://pkg.go.dev/golang.org/x/text/unicode/norm) (#77) | As regras de categoria e de descrição comparam o texto sem acento, como o `Normalize(NormalizationForm.FormD)` do C#, e a biblioteca padrão do Go não tem normalização Unicode. Mantido pelo time do Go, como o `x/sys`. Conferido contra o próprio C#: o gabarito de 79 casos de `internal/texto` (acentos, maiúsculas, emoji, hangul) é gravado por uma sonda que chama o código da API. |
| Ordem de texto da cultura (pt-BR) | [`golang.org/x/text/collate`](https://pkg.go.dev/golang.org/x/text/collate) (#77) | O C# ordena texto em memória (`OrderBy`/`ThenBy`) pela cultura do processo, com o ICU: acento e caixa só desempatam ("árvore" antes de "Bolsa"). A fatura do mês desempata os cartões assim, e o empate é comum (cartões com fatura zero). Mesmo módulo do `unicode/norm`. As tabelas do pacote são do CLDR 23 (Unicode 6.2), mais velhas que as do ICU: letra criada depois pode sair noutra posição. Conferido contra o próprio C#: o gabarito de 60 nomes de `internal/texto/testdata/ordem.json`. |
| Holerite em PDF | [`github.com/ledongthuc/pdf`](https://github.com/ledongthuc/pdf) ([0068](0068-holerite-em-pdf-lido-no-go-com-ledongthuc-pdf.md), #228) | Lê o texto com a posição de cada letra e abre PDF com senha (RC4 e AES-128), em Go puro, sem dependência (BSD-3, fork do `rsc.io/pdf`). Só no módulo Trabalho (`internal/modulos/trabalho/leitor`). Os PDFs dos testes são gerados na hora, com o `golang.org/x/text/encoding/charmap` (mesmo módulo do `unicode/norm`), só nos testes. |
| Logs | `log/slog` da biblioteca padrão | Log estruturado sem dependência ([blog do Go](https://go.dev/blog/slog)). |
| Certificado `.pfx` | [`software.sslmate.com/src/go-pkcs12`](https://pkg.go.dev/software.sslmate.com/src/go-pkcs12) (#73) | A biblioteca padrão não lê PKCS#12. Os geradores de hoje (`dotnet dev-certs` e `Export-PfxCertificate`) usam o formato antigo (3DES), que o `golang.org/x/crypto/pkcs12` também lê. Mas ele [está congelado](https://pkg.go.dev/golang.org/x/crypto/pkcs12) e não lê o `.pfx` cifrado com AES (testado: falha no SHA-256), que o OpenSSL 3 gera por padrão. O `go-pkcs12`, um fork mantido, lê os dois. Conferido: o Go leu o `.pfx` do `dotnet dev-certs` e serviu o mesmo certificado do .NET. *Julgamento a revisar:* trocar pelo `x/crypto/pkcs12` tira uma dependência de terceiro, ao custo de quebrar com um `.pfx` AES. Desde a #84, também grava o `.pfx` (`preparar-https`). |
| Janela e bandeja do gerenciador | [`github.com/wailsapp/wails/v3`](https://pkg.go.dev/github.com/wailsapp/wails/v3) ([0052](0052-gerenciador-em-go-com-bandeja-e-pagina-local.md)) | Só no gerenciador, não na API. Janela própria em HTML pelo navegador do sistema, acessível (a UI Automation vê os botões), sem porta aberta. Versão prévia: fixa no `go.mod`. |
| Ícone do `.exe` do gerenciador | [`github.com/tc-hib/go-winres`](https://github.com/tc-hib/go-winres) ([0052](0052-gerenciador-em-go-com-bandeja-e-pagina-local.md)) | Ferramenta de build, não vai no executável: gera o recurso `.syso` com o ícone e a versão do Windows. |

Dependência fora desta lista precisa de atualização desta ADR, ou de uma ADR nova.

## Consequências

- **Positivas:**
  - cada passo foi pequeno, em uso real e reversível pela configuração;
  - no fim, um executável sem runtime: o `.msi` caiu de 53 MB para 13 MB, a pasta instalada de 150 MB para 35 MB, e a memória do site de ~150 MB para ~18 MB (#85, #86);
  - o `sqlc` torna o esquema a fonte única das consultas, e o back-end ganhou testes, a começar pelo contrato.
- **Negativas:**
  - **reescrita grande**, numa linguagem que o time estava aprendendo;
  - **na transição:** dois runtimes juntos, um salto a mais em cada rota repassada e o risco de regra repetida divergir ou de formato errado entre os dois escritores;
  - **na inversão, todos foram deslogados** e informaram os segredos de novo, uma vez. Voltar à tag pede informá-los outra vez (o .NET não lê o `v2:`);
  - **domínios novos na instalação:** `go.dev`, `dl.google.com` e `proxy.golang.org` ([0011](0011-ferramentas-de-instalacao-portateis-sem-python.md));
  - **pegadinhas da linguagem** (RE2, decimal entre aspas no JSON, `sqlc` menos maduro no SQLite), registradas no [`estilo.md`](../../.claude/skills/go-no-projeto/referencias/estilo.md) e no README do banco;
  - **sem o .NET, não há referência** para conferir um comportamento "como era": o contrato e os gabaritos guardam o de 03/11/2026, e o resto só rodando a tag.

## Atualizações

Uma linha por atualização; o texto inteiro está no histórico do git (acima) e no CHANGELOG.

- **2026-11-02, #85 parte 1:** a troca pelos scripts (`--backend go|dotnet`), com cópia do banco antes e volta automática, ainda desligada por padrão.
- **2026-11-02, #85 parte 2:** o Go sozinho roda todas as tarefas, e o segredo gravado pelo .NET aparece como não configurado.
- **2026-11-02, #85 parte 4:** a inversão. O Go na frente vira o padrão, na 3.0.0, depois do ensaio com o banco real (92 de 93 telas iguais).
- **2026-11-03, #86 parte 2:** a instalação sem o .NET; sem volta pelos scripts, a volta é a tag `v-ultimo-dotnet`. Quem vinha da 2.x ficou sem a cópia "antes do Go", só com a de toda atualização.
- **2026-11-03, #86 partes 3 a 6:** o .NET saiu do gerenciador (#616), dos scripts e do `.msi` (#617), da CI (#620) e do código (#625, #626), e a pasta virou `src/backend` (#627). A migração terminou.
- **2026-11-15, #710:** as mensagens de validação que a API monta (`internal/plataforma/respostas`) saem em português, sem o `traceId` nem o nome do DTO do C#, e o JSON quebrado tem uma mensagem só. O formato (`type`, `title`, `status`, `errors`, `{"mensagem"}`) não muda; os esperados do contrato mudaram de propósito, só nessas mensagens.
