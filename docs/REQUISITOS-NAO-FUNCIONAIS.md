# Requisitos não funcionais

Quanto o sistema pode gastar e o que ele garante, com número e meta. Mudou algo que mexe numa meta? Meça antes e depois, e escreva no PR.

**Situação de cada meta:**
- **Conferida:** a CI ou um teste confere a cada mudança.
- **Medida:** há um número, mas ninguém confere sozinho.
- **Proposta:** vale como ponto de partida, até ser combinada com quem mantém o projeto (#470).
- **Sem medida:** ainda não foi medida.

## Eficiência

As metas de memória e de disco valem **para o Go no fim** (#86): antes disso, o .NET continua junto, e os números só pioram ou ficam iguais.

| Atributo | Medida | Hoje | Meta | Como medir | Situação |
|---|---|---|---|---|---|
| Memória da API em uso | MB do processo, ocioso e no pico de uma sincronização | ~225 MB (21/10/2026, 2.23, Windows, `.msi`) | Até 60 MB | Método da #86: 5 minutos depois de subir, sem requisições, e durante uma sincronização, com 3 leituras | Proposta. O topo do palpite do épico #70 |
| Tamanho da instalação | MB e arquivos de `%ProgramFiles%\FinancasLocal` e de `publicado/`, sem os dados | 157 MB (mesma medição) | Até 40 MB | Método da #86 | Proposta. O topo da estimativa do épico #70 |
| Tempo de partida | Segundos do início do serviço até `GET /api/auth/estado` responder | Sem medida | A definir depois da primeira medição | Com um banco de referência, 3 vezes | Sem medida |
| Tempo de resposta das telas principais | Tempo do Painel, de Transações e de Investimentos | Sem medida | A definir depois da primeira medição | Com 5 anos de transações fictícias | Sem medida |

## Segurança e privacidade

| Atributo | Medida | Meta | Como medir | Situação |
|---|---|---|---|---|
| Exposição na rede | Endereço em que o sistema escuta | Só `127.0.0.1` | `Kestrel` no `appsettings.json` e, no Go, `atras_do_dotnet_test.go` | Conferida no Go; no .NET, medida na configuração |
| Saídas de rede | Domínios acessados | Só os da [tabela de saídas](SEGURANCA.md#o-que-sai-para-a-internet) | Cada domínio novo pede uma ADR | Conferida na revisão do PR |
| Dado mínimo enviado | O que vai em cada saída | Quantidades, valores e dados pessoais não saem | [Dados pessoais](DADOS-PESSOAIS.md) e [modelo de ameaças](MODELO-DE-AMEACAS.md) | Conferida na revisão do PR |
| Scripts de terceiros no site | Domínios na CSP | Nenhum | CSP da API nos testes do navegador | Conferida: testes no navegador ([ADR 0041](adr/0041-testes-no-navegador-com-playwright.md)) |
| Vulnerabilidades | Dependências e código | Nenhuma nova | `govulncheck` e [CodeQL](adr/0054-analise-de-seguranca-do-codigo-com-o-codeql.md) | Conferida na CI |

## Confiabilidade

| Atributo | Medida | Meta | Como medir | Situação |
|---|---|---|---|---|
| Nenhuma atualização apaga dados | Dados depois de atualizar | Todos ficam | Os workflows "Teste de instalação" atualizam em cima de uma instalação, mas não comparam os dados | Medida: nada compara os dados antes e depois |
| Cópia antes de mudar o banco | Cópia em `dados/backups` | Sempre que a estrutura mudar, guardando as 5 mais recentes | `EsquemaBanco` | Medida no código |
| Voltar de uma cópia | Restaurar e o site abrir | Funciona seguindo o [guia](OPERACAO.md#restaurar-uma-cópia) | Passo "Restaurar a cópia" dos workflows | Conferida na CI, nos scripts e no `.msi` |
| Sem internet | O que continua funcionando | O site abre e mostra o que está no banco. Câmbio guardado continua valendo. Sincronização e cotações param até a internet voltar | À mão | Sem medida |
| Go fora do ar | Rotas do Go | 503 com mensagem, e reinício sozinho | Passo 9 do `scripts/ci/testar-go-atras-do-dotnet.sh` | Conferida na CI |

## Portabilidade

| Atributo | Medida | Meta | Como medir | Situação |
|---|---|---|---|---|
| Sistemas | Onde instala | Windows e Linux, com os mesmos scripts | Workflows "Teste de instalação", do zero em máquina limpa | Conferida na CI |
| Versões mínimas | .NET 10, Go 1.27, Angular 22 | As que o projeto compila | `Financas.Api.csproj`, `go.mod` e `package.json` | Conferida na CI |

## Manutenibilidade

| Atributo | Medida | Meta | Como medir | Situação |
|---|---|---|---|---|
| Tempo dos testes do Jest | Segundos | Até 20 s por arquivo e 150 s por suíte | `scripts/ci/verificar-tempo-testes.py` | Conferida na CI |
| Tempo dos testes do Go | Segundos | Até 20 s por teste e 120 s por suíte | `scripts/ci/verificar-tempo-testes-go.py` | Conferida na CI |
| Cobertura do Go | % por pacote | Só sobe (#370); pacote novo nasce com 80% | `cobertura-minima.txt` e `verificar-cobertura-go.py` | Conferida na CI |
| Mutação do Go | % por pacote | O piso de `mutacao-go.txt` só sobe (#373) | `scripts/ci/mutacao-go.py` | Conferida na CI, toda segunda e quando muda |
