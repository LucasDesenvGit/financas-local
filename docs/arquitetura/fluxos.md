# Fluxos principais

Como as partes conversam nos caminhos que cruzam o sistema. Os nomes dos participantes são os do [diagrama de Contêineres](../ARQUITETURA.md#contêineres). O padrão está na [ADR 0055](../adr/0055-documentacao-de-arquitetura-em-diagramas-mermaid.md).

> **Data dos diagramas:** 01/11/2026, na fase "Go atrás do .NET" (#231). Cada fluxo diz quem atende hoje. Quando um assunto passa para o Go, o fluxo dele muda no mesmo PR.

## Login e sessão

**Quem atende:** a API .NET, até a inversão (#85).

```mermaid
sequenceDiagram
    actor Pessoa as Você
    participant Nav as Navegador
    participant API as API (.NET)
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
    Nav->>API: POST, PUT, PATCH ou DELETE, com cookie e X-XSRF-TOKEN
    API->>BD: o carimbo de segurança da sessão ainda vale?
    alt carimbo mudou (a senha foi trocada)
        API-->>Nav: 401: entre de novo
    else CSRF errado
        API-->>Nav: 400
    else tudo certo
        API-->>Nav: resposta
    end
```

- A senha fica com hash PBKDF2 (ASP.NET Identity), e os limites são parâmetros em **Configurações**.
- A sessão expira sem uso, e cada pedido renova o prazo.

## Pedido repassado ao Go

**Quem atende:** a API recebe, e o Go responde o que está em `Go:Prefixos`: câmbio, cripto, cadastros, contas, conferir com o banco e transações ([ADR 0049](../adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)).

```mermaid
sequenceDiagram
    participant Nav as Navegador
    participant API as API (.NET)
    participant Go as Back-end Go
    participant BD as Banco

    Nav->>API: pedido para um prefixo de Go:Prefixos
    Note over API: Exige login e, se grava, valida o CSRF
    alt Go fora do ar
        API-->>Nav: 503, com mensagem
    else Go no ar
        Note over API: Tira o cookie, o token CSRF e todo X-Financas-* de fora
        API->>Go: mesmo método, caminho, query e corpo,<br/>com X-Financas-Segredo e X-Financas-Usuario
        Note over Go: Sem o segredo interno: 401
        Go->>BD: lê e grava
        Go-->>API: resposta
        Note over API: Tira Server e Set-Cookie
        API-->>Nav: resposta
    end
```

## Partida da API

**Quem atende:** a API .NET. As seis rotinas de partida já existem em Go (`internal/partida`, #444), cada uma com a regra no pacote do assunto dela, mas só rodam com o Go na frente (opção `-rotinas-de-partida`, depois da inversão, #85). Atrás do .NET, quem as roda é o .NET: rodar nos dois seria acertar duas vezes.

```mermaid
sequenceDiagram
    participant Svc as Serviço do sistema
    participant API as API (.NET)
    participant BD as Banco
    participant Go as Back-end Go

    Svc->>API: inicia
    API->>BD: EsquemaBanco: se os scripts mudaram, cópia em dados/backups,<br/>depois os scripts embutidos, em modo WAL
    API->>BD: rotinas de partida: regras padrão, entradas nos cartões,<br/>transferências, cobranças e trocos sem dúvida
    Note over API: Uma falha nas rotinas vira aviso, e a API sobe
    API->>Go: sobe como processo filho, com o segredo pela entrada padrão
    loop enquanto a API estiver no ar
        Go-->>API: fora do ar?
        Note over API: As rotas dele respondem 503.<br/>Nova tentativa em 1 s, 2 s, 4 s...
    end
    Svc->>API: para
    API->>Go: fecha a entrada padrão, e o Go para junto
```

## Sincronização com a Pluggy

**Quem atende:** a API .NET, até a #83.

```mermaid
sequenceDiagram
    participant Tarefa as Tarefa agendada (API)
    participant Sinc as Sincronização (API)
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

## Atualização pelo gerenciador

**Quem atende:** o gerenciador do Windows, em C# ([ADR 0028](../adr/0028-gerenciador-do-windows-e-atualizacao-pelo-github.md)). A cópia do banco é feita pela própria API, ao iniciar a versão nova.

```mermaid
sequenceDiagram
    actor Pessoa as Você
    participant Ger as Gerenciador
    participant GH as GitHub
    participant MSI as Instalador
    participant API as API (.NET)
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
