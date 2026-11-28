# 0075. Configuração inicial só com o segredo do gerenciador: o primeiro usuário se cria por um link com token

- **Status:** Aceita (2026-11-28). Exigir o segredo foi decisão do Felipe (27/11/2026, #68 item 4); o caminho do Windows com `.msi`, o UAC uma vez, também (28/11/2026).
- **Data:** 2026-11-28 (issue #68, item 4)
- **Relacionadas:** [0061](0061-acesso-pelo-celular-na-rede-local.md) (o segredo do gerenciador), [0052](0052-gerenciador-em-go-com-bandeja-e-pagina-local.md); o [modelo de ameaças](../MODELO-DE-AMEACAS.md)

## Contexto

`POST /api/auth/configurar` cria o único usuário e abre a sessão. Ele fica **aberto até o primeiro uso**: logo depois de instalar, qualquer programa do computador (ou de outro usuário dele) que chegue a `127.0.0.1:5443` cria o usuário antes do dono e passa a ser o dono do sistema. Com o acesso pelo celular ligado antes da configuração (o gerenciador liga sem login, ADR 0061), o mesmo vale para a rede de casa.

O que já existe: `dados/chaves/gerenciador-celular.chave`, um segredo de 32 bytes que o site cria ao subir e o gerenciador lê para ligar o acesso pelo celular. No Windows, a pasta só abre para o SYSTEM e os administradores; no Linux, para a conta do serviço (0600).

## Alternativas consideradas

- **Senha de instalação digitada pela pessoa** (um código que o instalador mostra): mais um segredo para anotar, e o instalador precisa mostrá-lo e guardá-lo.
- **Aceitar o `configurar` só do `127.0.0.1`:** já é assim, e não resolve: o atacante é um programa do próprio computador.
- **O gerenciador cria o usuário** (pede a senha na janela dele): a senha passaria por outro processo e por uma segunda tela de cadastro.
- **Usar o segredo do gerenciador no cabeçalho** (a decisão de 27/11 pede o segredo): vale, mas o segredo é permanente, e o link abre no navegador, com o texto na linha de comando que outros usuários do Linux veem (`ps`). Por isso, o link leva um **token derivado**, de 10 minutos. **Escolhida.**

## Decisão

- **O token** é `<instante>.<HMAC-SHA256("configurar|<instante>", segredo)>`. Vale 10 minutos (e aceita 1 minuto de relógio adiantado). Não revela o segredo e não é guardado em lugar nenhum.
- **A API** (`autenticacao`, `configuracao.go`): o `POST /api/auth/configurar` exige o cabeçalho `X-Configurar-Token` válido; sem ele, 403 e a mensagem de como abrir o site, antes de ler o corpo. Sem segredo carregado, ninguém configura. O `GET /api/auth/estado` continua público.
- **O link** é `https://localhost:5443/entrar?configurar=<token>`. O Angular guarda o token, tira-o da barra de endereço e o manda no cabeçalho. Sem o token, a tela do primeiro acesso só explica como abrir o site.
- **Quem gera o link:**
  - o **gerenciador**: "Abrir o site", enquanto o site diz `configurado: false` e o segredo pode ser lido, abre o link (`internal/primeiroacesso`). No Linux ele lê a pasta direto; no Windows, pelo UAC (abaixo);
  - o subcomando **`financas-go link-de-configuracao -chaves <pasta>`**, que os scripts de instalação (Linux e Windows) rodam no fim e mostram o link, e que serve para pedir outro link vencido. Quem o roda precisa ler a pasta das chaves, e ele nunca cria o segredo.
- A fórmula do token está no site e no gerenciador (outro módulo Go, que não importa o site), com o mesmo valor de referência nos dois testes.

## Consequências

- **Positivas:** fecha o item 4 do #68: quem não lê `dados/chaves` não cria o usuário. Sem estado novo, sem coluna nova, sem saída de rede.
- **Negativas:**
  - mais um passo na primeira vez: abrir pelo link (gerenciador ou instalação), e não digitar `localhost:5443`. Quem digita o endereço vê a explicação;
  - no Linux, qualquer programa da conta do usuário lê o segredo (é a mesma conta do serviço): a proteção é contra **outros usuários** do computador, não contra o malware da própria conta (*julgamento*, o mesmo limite do ADR 0061);
  - o link aparece na linha de comando do navegador por alguns segundos: outro usuário do Linux que vigiasse `ps` nessa hora o pegaria, e teria até 10 minutos para criar o usuário primeiro. O dono perceberia: a tela diria "já configurado" (*julgamento*: risco baixo e curto; trocar o token do link por uma troca de uma vez só, com cookie, fecharia isso);
  - o token não é de uso único: vale por 10 minutos, mas o `configurar` já só funciona uma vez.
- **Na prática:**
  - os testes de contrato e de CI mandam o token (`tokenConfiguracao` nos casos; os workflows e o `medir-desempenho.py` rodam o subcomando);
  - **Windows com `.msi` (decisão do Felipe, 28/11/2026): o UAC, uma vez.** A pasta das chaves só abre para administradores e o gerenciador roda sem elevação. Enquanto o site não tem usuário e o gerenciador não lê o segredo, "Abrir o site" pede o UAC, uma instância elevada (`--primeiro-acesso <canal>`) gera o token e o devolve, e **quem abre o navegador é a instância sem elevação**. Com o usuário criado, o UAC nunca mais aparece. Recusou o UAC ou deu erro: aviso na bandeja ("não dá para gerar o link...") e a página de sempre.
  - **O canal de volta é um pipe nomeado**, criado pela instância sem elevação (`primeiroacesso/canal_windows.go`): nome aleatório de 128 bits, ACL só do SID do usuário (o elevado do UAC tem o mesmo SID), uma instância (`FIRST_PIPE_INSTANCE`: ninguém ocupou o nome antes), sem clientes da rede, só entrada. O elevado só escreve em nome do formato certo e só envia uma linha, o token (nunca o segredo); quem lê só aceita esse formato, em até 256 bytes. Não há arquivo nem caminho escolhido por quem não é elevado.
  - **Alternativas do canal de volta:** o handle herdado não existe com `ShellExecuteEx` + `runas` (ele não herda handles); um arquivo gravado pelo elevado num caminho dado pela instância comum deixaria um processo sem privilégio escolher onde o administrador escreve; a área de transferência seria visível a qualquer programa e ficaria com o token depois. **Outras opções para o Windows, descartadas:** um segredo próprio do primeiro acesso, legível pelo usuário que instalou (sem UAC, mas um segredo, um arquivo e uma ACL novos no `.msi`); o instalador mostrar o link na tela final (o `configurar.ps1` roda como SYSTEM, sem acesso à tela do usuário).
  - **Limite do UAC:** um programa da sua conta pode abrir o `--primeiro-acesso` com um pipe dele e esperar que você aceite o aviso do UAC (que mostra o gerenciador), e então teria o token (*julgamento*: é o mesmo malware da conta, fora do escopo do modelo de ameaças). Se o aviso vier de um administrador diferente (credenciais de outra conta no UAC), o SID não confere, o pipe recusa e cai no aviso de erro. Não foi conferido num Windows de verdade.
