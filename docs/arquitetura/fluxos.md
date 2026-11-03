# Fluxos principais

Como as partes conversam nos caminhos que cruzam o sistema. Os nomes dos participantes são os do [diagrama de Contêineres](../ARQUITETURA.md#contêineres). O padrão está na [ADR 0055](../adr/0055-documentacao-de-arquitetura-em-diagramas-mermaid.md).

> **Data dos diagramas:** 03/11/2026, com o .NET já fora (#86): quem atende tudo é o site em Go. Quando um fluxo muda, o diagrama dele muda no mesmo PR.

## Login e sessão

**Quem atende:** o site (`internal/autenticacao`, #75).

```mermaid
sequenceDiagram
    actor Pessoa as Você
    participant Nav as Navegador
    participant API as Site (Go)
    participant BD as Banco

    Nav->>API: GET /api/auth/estado
    API-->>Nav: estado e cookies de CSRF
    Pessoa->>Nav: login e senha
    Nav->>API: POST /api/auth/entrar, com X-XSRF-TOKEN
    Note over API: Limite de tentativas por minuto: 429
    API->>BD: procura o usuário
    alt usuário não existe
        Note over API: Verifica um hash fictício,<br/>para o tempo não revelar nada
        API-->>Nav: 401 "Usuário ou senha inválidos."
    else bloqueado
        API-->>Nav: 429, com os minutos que faltam
    else senha errada
        API->>BD: conta a falha, bloqueia no limite, auditoria
        API-->>Nav: 401 "Usuário ou senha inválidos."
    else senha certa
        API->>BD: zera as falhas, auditoria
        API-->>Nav: cookie de sessão __Host- e CSRF novo
    end

    Note over Nav,API: Daí em diante
    Nav->>API: qualquer pedido, com o cookie<br/>(e o X-XSRF-TOKEN, se for POST, PUT, PATCH ou DELETE)
    API->>BD: em todo pedido: o carimbo de segurança da sessão ainda vale?
    alt carimbo mudou (a senha foi trocada)
        API-->>Nav: 401: entre de novo
    else grava e o CSRF está errado
        API-->>Nav: 400
    else tudo certo
        API-->>Nav: resposta
    end
```

- A senha fica com hash PBKDF2, no formato do ASP.NET Identity que o .NET usava, para as senhas de antes continuarem valendo. Os limites são parâmetros em **Configurações**.
- A sessão expira sem uso, e cada pedido renova o prazo.

## Partida do site

**Quem atende:** o site, com a linha do serviço instalado (`financas-go -aplicar-esquema -rotinas-de-partida`).

```mermaid
sequenceDiagram
    participant Svc as Serviço do sistema
    participant API as Site (Go)
    participant BD as Banco

    Svc->>API: inicia
    API->>BD: -aplicar-esquema: se os scripts mudaram, cópia em dados/backups,<br/>depois os scripts embutidos, em modo WAL
    API->>BD: -rotinas-de-partida: regras padrão, entradas nos cartões,<br/>compras parceladas estornadas, transferências,<br/>cobranças e trocos sem dúvida
    Note over API: Uma falha nas rotinas vira aviso, e o site sobe
    Note over API: Começa a atender em 127.0.0.1:5443<br/>e liga as tarefas: câmbio, cripto e sincronização
    Svc->>API: para
    Note over API: Espera os pedidos em andamento,<br/>para as tarefas e fecha o banco
```

## Sincronização com a Pluggy

**Quem atende:** o site (`internal/sincronizacao`, #83).

```mermaid
sequenceDiagram
    participant Tarefa as Tarefa agendada (site)
    participant Sinc as Sincronização (site)
    participant Pluggy as Meu Pluggy / Pluggy
    participant BD as Banco

    loop a cada 5 min
        Tarefa->>BD: ligada, há conexão ativa e passou o intervalo?
    end
    Tarefa->>Sinc: sincroniza
    Sinc->>Pluggy: cria a sessão com o Client ID e o Secret
    Sinc->>BD: carrega as regras de categorização
    loop cada conexão ativa
        Sinc->>Pluggy: item: situação e validade do consentimento
        Sinc->>BD: guarda a situação (o Painel avisa quando o consentimento vai vencer)
        Sinc->>Pluggy: contas e faturas
        loop cada conta, página por página
            Sinc->>Pluggy: GET /v2/transactions, com o cursor
        end
        Sinc->>BD: grava, categoriza e aplica o tipo
        Sinc->>Pluggy: investimentos e empréstimos
        Sinc->>BD: grava e registra em SincronizacaoLog
    end
    Sinc->>BD: transferências, parceladas estornadas, cobranças e trocos sem dúvida
```

- O intervalo vem de **Configurações**, entre 1 e 48 horas, e o padrão é 6.
- Um erro ao classificar os trocos não desfaz a sincronização: o troco continua na conferência.
- A sincronização da tela e a da tarefa são a mesma instância, com uma trava de uma rodada por vez.

## Atualização pelo gerenciador

**Quem atende:** o gerenciador em Go ([ADR 0052](../adr/0052-gerenciador-em-go-com-bandeja-e-pagina-local.md)). A cópia do banco é feita pelo próprio site, ao iniciar a versão nova.

```mermaid
sequenceDiagram
    actor Pessoa as Você
    participant Ger as Gerenciador
    participant GH as GitHub
    participant MSI as Instalador
    participant API as Site (Go)
    participant BD as Banco

    Ger->>GH: última versão publicada (só leitura, sem enviar nada seu)
    GH-->>Ger: versão, .msi e SHA256SUMS.txt
    Ger-->>Pessoa: "Tem versão nova"
    Pessoa->>Ger: Atualizar
    Ger->>GH: baixa o .msi
    alt SHA256 não confere
        Ger-->>Pessoa: apaga o arquivo e avisa: nada foi instalado
    else SHA256 confere
        Ger->>MSI: abre o .msi, com log, e fecha
        Pessoa->>MSI: confirma (UAC)
        MSI->>API: para o serviço, troca os arquivos e inicia
        API->>BD: se a estrutura mudou, cópia em dados/backups antes de mudar
        MSI-->>Pessoa: oferece abrir o gerenciador
    end
```

Se algo falhar: [Atualização que falhou](../OPERACAO.md#atualização-que-falhou).
