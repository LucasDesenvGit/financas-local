# 0075. Configuração inicial só com o segredo do gerenciador: o primeiro usuário se cria por um link com token

- **Status:** Proposta (2026-11-28). A decisão de exigir o segredo é do Felipe (27/11/2026, #68 item 4); o caminho do Windows (`.msi`) falta decidir, abaixo.
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
  - o **gerenciador**: "Abrir o site", enquanto o site diz `configurado: false` e o segredo pode ser lido, abre o link (`internal/primeiroacesso`). Funciona no Linux, em que ele lê a pasta direto;
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
  - **Pendente, Windows:** com o `.msi`, a pasta das chaves só abre para administradores, e o gerenciador roda sem elevação: ele **não consegue** gerar o link, e "Abrir o site" abre a página que manda abrir pelo gerenciador (um beco sem saída). Opções, para o Felipe escolher *(inferido; nada disso foi confirmado)*:
    1. **UAC no "Abrir o site", só enquanto não houver usuário:** uma instância elevada (`--primeiro-acesso`) lê o segredo e devolve o link ao gerenciador sem arquivo (por exemplo, o gerenciador elevado grava o link na área de transferência). Um aviso do UAC a mais, uma vez.
    2. **Um segredo próprio do primeiro acesso, legível pelo usuário que instalou:** o `.msi` (`configurar.ps1`) o grava com ACL só para esse usuário e para o SYSTEM, o site o aceita no lugar do segredo do gerenciador, e ele some quando o usuário é criado. Sem UAC, mas é um segredo, um arquivo e uma ACL novos.
    3. **O instalador mostra o link na tela final:** o `configurar.ps1` roda como SYSTEM, com o usuário comum sem acesso à tela dele; teria de ser um passo à parte do WiX.
