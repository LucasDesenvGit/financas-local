# Back-end em Go

O site do Finanças Local: serve o Angular e a `/api` num executável só (`financas-go`), como serviço do sistema ([ADR 0049](../../docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)). Como ele roda (partida, login, segredos, tarefas, erros, banco): [docs/ARQUITETURA.md, "Back-end"](../../docs/ARQUITETURA.md#back-end--srcbackend); os pacotes, agrupados em componentes: [Componentes do site](../../docs/arquitetura/componentes.md); o padrão de um assunto e as áreas: [`internal/README.md`](internal/README.md).

`cmd/financas` é a entrada (as opções, `-h` lista; o subcomando `preparar-https` da instalação, em `https.go`); `cmd/versao`, o cálculo da versão (ADR 0070); `internal/`, um pacote por assunto; `testes/contrato`, a regressão do site ([README](testes/contrato/README.md)).

## Rodar

Go 1.27.4 ou mais novo (os anteriores têm falhas de segurança na biblioteca padrão). Em desenvolvimento, com uma **cópia** do banco, ao lado do site instalado (que fica na 5443):

```bash
cd src/backend
go run ./cmd/financas -endereco 127.0.0.1:5444 -wwwroot ../frontend/dist/financas-web/browser -certificado-temporario \
  -banco <cópia>/FinancasLocal.db -aplicar-esquema
```

- `-wwwroot` é o build do Angular; com o `npm start`, use `-endereco localhost:7443` (a porta do `proxy.conf.json`) e qualquer pasta (`internal/plataforma/servidor/testdata/wwwroot`). `-pasta` usa o `.pfx` e os caminhos de uma instalação.
- `-aplicar-esquema` e `-rotinas-de-partida` (a linha do serviço liga os dois: `exe_servico` no `3-publicar-atualizar.sh`, `$binPath` no `instalar-windows.ps1`) só com uma cópia do banco. `-inicio-do-relogio` é só para teste (#363).

## Rota nova

1. O handler no pacote do assunto, com as dependências pelo construtor; o pacote exporta `Registrar(mux, <só o que ele usa>)` (o exemplo é o `internal/plataforma/saude`).
2. **Uma linha** em `internal/api/api.go`: `<assunto>.Registrar(mux, d.DB, d.Log, d.Relogio)`. Conflito ali no merge é sempre "fique com as duas linhas". Hora vem do `d.Relogio`, nunca do `time.Now()`.
3. Os casos de contrato, num número livre de `20` a `89` ([como](testes/contrato/README.md#adicionar-um-caso)), e os testes do assunto com o [`testeapoio`](internal/plataforma/testeapoio/README.md) (ADR 0051).
4. Tarefa em segundo plano nova: entra em `tarefas.Conhecidas` e no `iniciarTarefas` do `cmd/financas`.

## Antes de subir (push)

```bash
gofmt -l .                 # não pode listar nada
go vet ./...
python3 ../../scripts/ci/go/verificar-regras-go.py   # float64, time.Now() e time.Sleep de teste só com a exceção escrita (#457)
go test -race ./...
go generate ./... && git diff --exit-code -- .   # código gerado e cópia dos scripts do banco em dia
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
# e com GOOS=windows (os _windows.go, #449): instale antes, porque "GOOS=windows go run" compila o govulncheck para Windows
go install golang.org/x/vuln/cmd/govulncheck@v1.9.0 && GOOS=windows "$(go env GOPATH)/bin/govulncheck" ./...
GOOS=windows go build ./...   # o executável do Windows compila sem cgo
# De ponta a ponta: o contrato inteiro contra o Go (#72) e o site ligando como o serviço (#309)
bash testes/contrato/rodar.sh
bash ../../scripts/ci/ligar-api.sh
```

Pacote com regra: `python3 ../../scripts/ci/go/mutacao-go.py internal/<area>/<assunto>` (teste de mutação, ~1 min). Os jobs da CI, o contrato, a cobertura e a mutação: [CONTRIBUINDO.md, "Testes do back-end (Go)"](../../CONTRIBUINDO.md#testes-do-back-end-go).

Todo PR com Go traz a seção "🎓 Aula rápida" na descrição.
