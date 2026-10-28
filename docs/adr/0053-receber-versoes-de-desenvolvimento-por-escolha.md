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
