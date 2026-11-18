# 0053. Receber versões de desenvolvimento por escolha, no gerenciador em Go

- **Status:** Aceita
- **Data:** 2026-10-28 (issue #420, pedido do @JulioCezarSP)
- **Relacionadas:** substitui em parte a [0040](0040-atualizar-pelo-develop-so-na-instalacao-de-desenvolvimento.md) (o tipo de versão que o gerenciador procura deixa de vir só da versão instalada); [0028](0028-gerenciador-do-windows-e-atualizacao-pelo-github.md) (atualização pelo GitHub Releases, com SHA256); [0052](0052-gerenciador-em-go-com-bandeja-e-pagina-local.md) (gerenciador em Go); [0032](0032-controles-padrao-interruptor-e-campo-de-valor.md) (interruptor para o que vale na hora)

## Contexto

Pela 0040, o gerenciador procura versão nova conforme o que está instalado: a versão publicada (`X.Y.Z`) só olha o Release mais recente, e a de desenvolvimento (`X.Y.Z-dev.N`) só olha o pré-lançamento "desenvolvimento". Para passar de uma para a outra, é preciso baixar o outro `.msi` à mão no GitHub.

Ao portar a atualização para o gerenciador em Go (#420), o @JulioCezarSP pediu para poder ligar as versões de desenvolvimento pelo próprio gerenciador, para testar sem baixar o `.msi` à mão. A 0040 tinha recusado um "interruptor para todos", porque ele levaria código não lançado a quem instalou uma versão publicada (o risco que a 0028 recusou).

## Alternativas consideradas

- **Manter a 0040:** o tipo de versão vem só do que está instalado. Prós: ninguém recebe código não lançado sem baixar o `.msi` de desenvolvimento. Contras: trocar de tipo continua sendo à mão, no GitHub.
- **Interruptor ligado para todos, ou sem aviso:** o que a 0040 recusou. Contras: leva código não lançado a quem não pediu.
- **Interruptor desligado por padrão, com explicação:** só quem liga recebe as versões de desenvolvimento. O download continua conferindo o SHA256.

## Decisão

O gerenciador em Go tem, em **Mais opções**, o interruptor **"Receber versões de desenvolvimento"**, com um ⓘ que explica o que são e o que acontece ao desligar.

| Interruptor | O que "Procurar versão nova" faz |
|---|---|
| Ligado | procura no pré-lançamento "desenvolvimento" (`releases/tags/desenvolvimento`) |
| Desligado | procura só no Release mais recente (`releases/latest`) |
| Sem escolha gravada | como na 0040: o tipo da versão que está rodando |

- **Só no `.msi`:** pelos scripts, o gerenciador não procura versão nova (0040 continua valendo).
- **A versão nova continua precisando ser maior que a que está rodando** (`X.Y.Z`, e depois o `N`), e o `.msi` só abre se o SHA256 bater com o `SHA256SUMS.txt` da mesma versão (0028).
- **A escolha fica no registro do usuário** (`HKCU\Software\FinancasLocal\Gerenciador`, `ReceberDesenvolvimento`), ao lado das outras preferências do gerenciador; trocar a escolha esquece a procura anterior.
- **Interruptor, e não caixa de marcar:** a escolha vale na hora, na próxima procura (0032, Kendrick/NN/g).

## Consequências

- **Positivas:**
  - quem testa recebe as versões de desenvolvimento pelo próprio gerenciador, sem baixar o `.msi` à mão;
  - quem não liga continua como na 0040, sem receber código não lançado.
- **Negativas:**
  - **Quem liga recebe código menos testado:** as versões de desenvolvimento saem a cada merge no `develop`, depois dos testes de instalação da CI (0029), mas antes de virar versão publicada. O ⓘ diz isso, sem jargão.
  - **Voltar não é imediato:** quem está na `2.45.0-dev.5` e desliga só recebe versão publicada quando sair uma maior que a `2.45.0`. O Windows não deixa o `.msi` instalar uma versão mais velha por cima. O ⓘ diz isso também.
  - **Instalação de desenvolvimento pelo `.msi` pode receber a versão publicada:** com o interruptor desligado, ela passa a olhar o Release mais recente (antes, pela 0040, só o pré-lançamento).
  - **O gerenciador em C# continua como na 0040** até ser trocado pelo em Go (#86): os dois coexistem com regras diferentes nesse ponto.
- **Na prática:**
  - `src/gerenciador-go/versao_nova.go` decide o tipo pela preferência (`preferencias.ReceberDesenvolvimento`), com o tipo da versão que roda como padrão;
  - nenhum domínio de saída novo: `api.github.com` e `github.com`, como na 0028.

## Atualização de 2026-11-16: o número de cada merge é calculado (ADR 0070, #721)

- **O que muda:** cada merge no `develop` continua tendo uma versão `X.Y.Z` própria, o que o gerenciador compara (a nova tem de ser maior que a que roda). Só deixa de ser um número gravado pelo PR no `Directory.Build.props`: o `go run ./cmd/versao` o calcula da base e dos merges, e o `versao.yml` o dá ao `gerar-msi.ps1`, que o grava no site e no gerenciador como antes (`X.Y.Z-dev.<execução>` fora da tag).
- **O que não muda:** o formato do rótulo, as notas por versão de desenvolvimento (`### X.Y.Z — `, que o `notas.go` separa) e a regra "só vale se for maior".
- **Instalação pelos scripts:** a versão vem do mesmo cálculo, no clone. Numa cópia `.zip` (sem o histórico do git), sai a base com o sufixo `-sem-git` (`3.27.0-sem-git`), e o gerenciador a lê pelo `X.Y.Z`.
- **"Atualizar para a última do develop" (0040):** o "O que mudou" deixa de ler o `Directory.Build.props` e passa a pedir o número e as seções do que ainda não foi publicado ao próprio clone (`go run ./cmd/versao`), com a versão de antes vinda do site instalado.
