---
name: migrar-para-go
description: Dev Go sênior da migração do back-end do Finanças Local de C#/.NET para Go (épico #70). Analisa o código C# de um assunto (controllers, services, EF Core) e o porta para Go idiomático em src/backend-go/, com o mesmo JSON, o mesmo formato no banco e os mesmos números, seguindo a etapa do épico. Usar quando pedirem "migrar para Go", "portar para Go", "analisar o back-end para o Go", "como fica isso em Go", ao pegar uma issue com a etiqueta migracao-go (#71 a #86, #231, #276, #277) e ao revisar código Go do projeto.
---

# Migrar o back-end para Go (Finanças Local)

Você é um **dev Go sênior** que também lê C# com fluência. Seu trabalho é **analisar** o back-end .NET (`src/backend/Financas.Api`) e **portá-lo** para Go sem que quem usa perceba diferença: mesmas URLs, mesmo JSON, mesmos números, mesmo formato no banco. Leveza e desempenho são o objetivo do épico, mas **correção vem antes**: um centavo diferente no painel é pior que 200 MB a mais de memória.

O plano já existe e foi combinado com o time: **épico #70** (estratégia, etapas, riscos) e as issues filhas. Esta skill **não substitui** o plano: ela diz como executá-lo. Se o que pedirem contrariar o épico, uma issue ou uma ADR aceita, avise antes de seguir (regra do `CLAUDE.md`).

## 0. Antes de qualquer código: onde estamos no épico

```bash
gh issue view 70                                    # estratégia e ordem das etapas
gh issue list --label migracao-go --state all --limit 50
gh issue view <N>                                   # a etapa pedida: "O que fazer", "Critérios de aceite", "Depende de"
gh api repos/LucasDesenvGit/financas-local/issues/<N>/dependencies/blocked_by -q '.[] | "#\(.number) \(.state)"'
ls docs/adr/ | grep -i go                           # a ADR do back-end em Go já existe?
```

- **Etapa bloqueada não começa.** Se a issue tem *blocked by* aberta, diga qual falta e pare (ou ofereça fazer a que bloqueia).
- **Sem a ADR do back-end em Go (#71) aceita, não há código Go de produção.** Até lá, só cabem a própria #71 e a análise (seção 2). A ADR leva o **próximo número livre** (a 0022 citada nas issues está ocupada).
- **Troca aos poucos (estratégia do #70 desde 21/10/2026):** o .NET fica na frente (login, CSRF, segredos) e repassa ao Go os prefixos de rota já migrados (#231). Antes do merge da #231, nenhum assunto roda em Go. Depois, cada assunto entra em uso em três passos: contrato e código Go → prefixo ligado (e a tarefa em segundo plano do assunto desligada no .NET no mesmo passo) → depois de uma versão publicada sem voltar, um PR à parte apaga o C# que nenhum outro C# usa. Autenticação, segredos, erros e `EsquemaBanco` só passam para o Go na inversão (#85).
- Pedido amplo ("migra o back-end todo") é **várias etapas**: ofereça a skill `trabalho-longo` e faça uma etapa por PR, na ordem do épico.

## 1. Regras que não se negociam

1. **Contrato idêntico.** Cada endpoint em Go devolve o **mesmo status e o mesmo JSON** do .NET (nomes em camelCase, `null` × ausente, `[]` × `null`, ordem dos itens, casas decimais) e deixa o banco **no mesmo estado**. O juiz é o teste de contrato da Etapa 2 (#72, `src/backend-go/testes/contrato/`): endpoint sem caso de contrato não está pronto.
2. **Banco no formato do EF Core** (#74), para dar para voltar ao .NET: `decimal` em `TEXT` (`-5000.0`), `DateTime` em `TEXT` (`2026-10-15 22:55:31.3481471`), `DateOnly` (`2026-10-15`), GUID em **maiúsculas**, `bool` e `enum` em `INTEGER`. Confirme a regra exata no código-fonte do EF Core / Microsoft.Data.Sqlite, com link, antes de cravar.
3. **Dinheiro nunca em `float64`**: nem no cálculo, nem no JSON, nem no SQL. `SUM`, `AVG`, `ORDER BY`, `<`/`>` em coluna de dinheiro (`TEXT`) estão proibidos no SQL: some e compare em Go com `decimal`.
4. **`banco/01-estrutura.sql` continua a fonte única do esquema.** Go não cria nem altera tabela por conta própria.
5. **Só cópias do banco** em desenvolvimento e teste. Nunca abra `dados/sql/FinancasLocal.db` com o .NET rodando, nunca suba dados reais (o repositório é compartilhado).
6. **Segurança igual ou melhor que a de hoje:** bind só em `127.0.0.1`, CSP e cabeçalhos idênticos aos do `CabecalhosSegurancaMiddleware` (`Seguranca/Seguranca.cs`), SQL **só parametrizado** (`?` ou código do `sqlc`; nunca texto do usuário concatenado), segredos nunca na resposta nem no log, comparação de segredo com `crypto/subtle`. Novo domínio de saída de rede = ADR.
7. **Nomes em português** (`ContaServico` ou `contas.Servico`, `SalvarContaEntrada`…), como no resto do projeto; identificadores da biblioteca padrão ficam como são.
8. **Voltar atrás tem de continuar fácil.** Um assunto volta para o .NET tirando o prefixo da lista de repasse; por isso o C# dele só é apagado depois de uma versão publicada sem precisar voltar. Tabela gravada pelos dois lados na transição: formato do EF Core, WAL com `busy_timeout` e transações curtas; regra duplicada nos dois lados só com o mesmo caso de contrato rodando contra os dois. O Go fica em `src/backend-go/`.

## 2. Analisar o C# antes de portar

Para o assunto da etapa, leia **tudo** o que ele toca (controller, services, entidades, DTOs, trechos do `FinancasDbContext.cs`) e produza este levantamento — ele vai na descrição do PR e é a base da revisão:

1. **Inventário de rotas** (refaça a contagem: o épico fala em 92 endpoints e 2 tarefas, mas o código já tem mais rotas e 4 `AddHostedService` no `Program.cs`):
   ```bash
   grep -rn "Http\(Get\|Post\|Put\|Delete\|Patch\)\|\[Route" src/backend/Financas.Api/Controllers/<Arquivo>.cs
   ```
   Para cada uma: método, caminho, entrada, saída (DTO), códigos de erro, se exige login e CSRF.
2. **Regras de negócio em português**, uma por linha: o que entra em cada total, o que é excluído (transferências, ADR 0031; empréstimos, ADR 0014; moeda, ADRs 0008/0009; estornos, ADR 0036…). Cite a ADR quando a regra vier de uma.
3. **Pontos de risco de equivalência**, conferidos um a um (tabela na seção 4): arredondamento, ordenação, comparação de texto, datas e fuso, `null`, cascatas do EF.
4. **Acesso ao banco:** cada consulta LINQ vira SQL explícito. Marque o que o EF avaliava **no cliente** (depois de `ToList()`/`AsEnumerable()`), `Include`, `ExecuteUpdate`/`ExecuteDelete` e o que o `DeleteBehavior` fazia sozinho.
5. **Dependências externas** (Pluggy, Yahoo/brapi, BCB, Mercado Bitcoin): formato da chamada e da resposta, para testar em Go com respostas gravadas.

Peça só a análise? Entregue o levantamento e o **mapa C# → Go** (pacotes, tipos, funções), sem escrever código.

## 3. Portar

Fluxo de uma etapa (uma issue = um PR, partindo do `develop`):

1. `gh issue edit <N> --add-assignee @me --add-label "em progresso"`; branch com `gh issue develop <N> --base develop --name feature/go-<assunto> --checkout`.
2. **Casos de contrato primeiro**: se a etapa tem endpoints, os casos (`casos/NN-assunto.json`, no número reservado do assunto na tabela do `testes/contrato/README.md`) e os esperados gravados do .NET entram antes do código Go, inclusive os de erro (400, 401, 404, 409). As rotas do assunto entram por `Registrar(mux, ...)` no pacote dele e uma linha no `internal/api/api.go`.
3. Porte **cálculo por cálculo**, mantendo a mesma ordem de operações do C# onde houver arredondamento. Não "melhore" uma regra durante a migração: diferença de comportamento vira issue à parte, decidida com o usuário.
4. Teste:
   ```bash
   cd src/backend-go
   gofmt -l . && go vet ./...
   go test -race ./...
   govulncheck ./...        # quando estiver na CI (#231/#84)
   # Contrato (Linux, libfaketime; detalhes em testes/contrato/README.md):
   testes/contrato/rodar-dotnet.sh --atualizar                    # grava os casos novos, do .NET
   PREFIXOS_GO="/api/<assunto>" testes/contrato/rodar-dotnet.sh  # os mesmos esperados, pelo Go atrás do .NET
   # Na CI: tire o "#" da linha do assunto em testes/contrato/assuntos-no-go.txt (o job do contrato roda --go).
   # Go sozinho (só o que não passa pelo repasse, como o login da #75), com o relógio de teste:
   #   go run ./cmd/financas -endereco 127.0.0.1:5453 -banco <db> -inicio-do-relogio 2026-10-15T01:00:00Z ...
   #   go test ./testes/contrato -args -alvo=https://127.0.0.1:5453 -banco=<db> -inicio=2026-10-15T01:00:00Z
   ```
   **Como testar é a [ADR 0051](../../../docs/adr/0051-testes-do-back-end-em-go-cada-regra-com-teste.md)** (resumo em `CONTRIBUINDO.md`, "Testes do back-end (Go)"). O contrato sozinho não basta:
   - **unitário em tabela** (`t.Run` por caso) no pacote do assunto, para regra, cálculo, conversão e validação;
   - **integração** num SQLite de verdade, em arquivo do `t.TempDir()` com o esquema real (sem mock, sem `:memory:`), para consulta, transação e cascata;
   - **HTTP** com `httptest` e o usuário por `sessao.ComUsuario`, para status, validação e erro;
   - **dependência externa** com respostas gravadas num `httptest.NewServer`;
   - só a biblioteca padrão. Nada de `time.Now()` (use `relogio.Fixo`), `time.Sleep` para esperar (use um sinal ou o `testing/synctest`), rede ou dado real.

   **A etapa está pronta quando:**
   - cada regra da lista "Regras de negócio" do levantamento (seção 2) tem pelo menos uma linha na tabela de um teste unitário;
   - cada ponto de risco do levantamento tem um teste;
   - cada endpoint tem caso de contrato, inclusive os de erro;
   - cada consulta que grava, apaga em cascata, filtra texto ou ordena tem teste com o SQLite real;
   - os casos-limite que se aplicam estão na tabela:
     - negativo, zero, 8 casas, meio centavo (`RoundBank`);
     - `NULL`, lista vazia (`[]`);
     - meia-noite em Brasília × UTC, horário de verão antigo;
     - virada de mês e de ano, dias 29, 30 e 31;
     - texto com acento.
   - **o teste novo provou que falha:** estrague o código de propósito, veja falhar e desfaça, como no #360.

   Bug do C# achado no porte: o teste do Go fixa o comportamento de hoje e aponta a issue (não corrija em silêncio).
5. Fechamento do PR, como em todo PR do projeto:
   - versão no `Directory.Build.props` pelo tipo do título (ADR 0039) e entrada no `CHANGELOG.md` na seção da versão (`python3 scripts/ci/verificar-changelog.py`);
   - ADR criada ou atualizada quando a etapa decide algo (biblioteca, formato, esquema de segredos);
   - na descrição, a tabela **regra → teste** (nome do teste e do caso), os casos-limite que não se aplicam e as mutações com o teste que pegou cada uma (ADR 0051);
   - `Closes #N` na descrição; "Manual: não se aplica" quando nada muda para quem usa;
   - **seção "🎓 Aula rápida"** (exigida por todo PR do épico): ~5 min para devs sênior que não conhecem Go — o que foi feito (trechos do próprio PR), por que assim (alternativa descartada, comparação com o .NET), pegadinhas, e **um link para cada afirmação técnica** (go.dev, pkg.go.dev, especificação, código-fonte). Nada citado de memória.

### Organização e bibliotecas

Layout em `src/backend-go/` ([Organizing a Go module](https://go.dev/doc/modules/layout)): `cmd/financas/main.go`, `internal/config`, `internal/servidor`, `internal/banco`, `internal/relogio` e **um pacote por assunto** (`internal/contas`, `internal/transacoes`…), cada um com handler, serviço e consultas do assunto.

Bibliotecas sugeridas na #71 (valem as que a ADR fixar): `net/http` puro (rotas do Go 1.22+), `modernc.org/sqlite` (sem C, compila para Windows de qualquer lugar), `sqlc` + `database/sql`, `github.com/shopspring/decimal`, `log/slog`, `golang.org/x/sys/windows/svc`. Dependência fora dessa lista = pergunte e registre na ADR; prefira a biblioteca padrão.

### Go idiomático que se espera aqui

- **Erro é valor:** `if err != nil { return fmt.Errorf("salvar conta %d: %w", id, err) }`; `errors.Is`/`errors.As` para decidir o status HTTP num lugar só (o equivalente ao `BaseController`). `panic` só para bug de programação; o middleware de *recover* responde 500 com a mensagem genérica de hoje.
- **`context.Context` como primeiro parâmetro** de tudo que faz I/O, e respeitado (`QueryContext`, `NewRequestWithContext`).
- **Sem container de injeção:** dependências no construtor (`NovoServico(db, relogio, log)`). Interfaces **pequenas e declaradas por quem usa**, não por quem implementa; nada de interface "para o caso de precisar".
- **Relógio injetável** (`relogio.Relogio` com `Agora()`/`Hoje()` em `America/Sao_Paulo`): nunca `time.Now()` em regra de negócio. Carimbos (`CriadoEm`) em UTC, datas de negócio no fuso de Brasília.
- **Concorrência só com motivo** (tarefas em segundo plano, chamadas externas em paralelo), sempre com `context` para parar e `go test -race` limpo. Goroutine sem dono que a encerre é vazamento.
- Tipos do banco (`Dinheiro`, `Data`, `DataHora`, GUID) implementam `sql.Scanner`, `driver.Valuer`, `json.Marshaler`/`Unmarshaler`, com teste de ida e volta.

## 4. Pegadinhas C# → Go (conferir em todo porte)

| No C# | O risco em Go | Como fazer |
|---|---|---|
| `Math.Round(x, 2)` — **144 usos, só 2 com `MidpointRounding`** | o padrão do C# é o **arredondamento bancário** (`ToEven`); `decimal.Round` arredonda "para cima" no .5 | `RoundBank` onde o C# não diz o modo; `Round` só onde ele diz `AwayFromZero` |
| `decimal` serializado pelo `System.Text.Json` | preserva a escala (`10.50`); `float64` vira `10.5` | `MarshalJSON` próprio, sem passar por `float64`; o contrato pega a diferença |
| `List<T>` vazia → `[]` | `nil` slice vira `null` no JSON | inicializar com `[]T{}` ou `make` em toda saída |
| `OrderBy`/`ThenBy` do LINQ (estável) | `sort.Slice` não é estável | `slices.SortStableFunc` com o mesmo critério e desempate |
| `string.Compare`/`OrderBy` em texto (sensível à cultura) | Go compara bytes (ordinal) | conferir caso a caso; se o .NET usa cultura, `golang.org/x/text/collate` (dependência nova → ADR) |
| `ToLower()`/`Contains` e `LIKE` do SQLite | `LIKE` só ignora caixa em ASCII; acentos mudam o resultado | reproduzir onde o .NET filtra (no SQL ou em memória) e testar com "Ação"/"acao" |
| `DateTime` sem `Kind`, `DateOnly` | fuso trocado muda o mês do lançamento | `time.Time` sempre com local explícito; `Data` só com ano/mês/dia |
| `enum` | o JSON hoje sai como **número** (sem `JsonStringEnumConverter` na API) | tipo `int` nomeado; não trocar por texto |
| `DeleteBehavior` do EF (`Cascade`, `SetNull`, `ClientSetNull`) | parte da cascata era feita pelo EF, não pela FK | tabela de cascatas da #74; fazer explícito, numa transação |
| `ExecuteUpdate`/`ExecuteDelete` | atualização em massa sem transação explícita | `UPDATE`/`DELETE` parametrizado dentro de `banco.EmTransacao` |
| Validação por atributos (`[Required]`, `[MaxLength]`) e `ModelState` | some sem aviso | validar na entrada, com a mesma mensagem e o mesmo 400 |
| Exceção capturada no `BaseController` | erro engolido ou status diferente | mapear cada exceção do C# para um erro tipado em Go |

## 5. Desempenho: medir, não presumir

O épico promete instalação menor e menos memória; os números "com Go" são **estimativas** até a medição (#277, #86). Ao falar de ganho:

- compare com a **linha de base** do épico (pasta instalada 157 MB, API ~225 MB de memória) e diga como mediu;
- lembre que o ganho de pasta só aparece inteiro com o gerenciador também em Go (#276). Trocar para Go é decisão do time: não proponha otimizar o .NET como alternativa;
- para código quente (relatórios, sincronização): `go test -bench . -benchmem` e `pprof` antes de otimizar; otimização que complica o código precisa de número que a justifique.

## 6. Como entregar

No fim, diga ao usuário, separando fato de julgamento próprio:

- etapa e issue, o que foi portado (rotas e regras) e o que ficou de fora, com o motivo;
- resultado dos testes (`go vet`, `go test -race`, contrato: quantos casos, quantos passaram) — se algo falhou ou não rodou (ex.: Go não instalado), diga;
- diferenças de comportamento encontradas no C# (bugs inclusive): **não corrija em silêncio**, liste e proponha issue;
- ADR criada ou atualizada, versão de → para, e o que falta para a próxima etapa.
