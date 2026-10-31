# Requisitos não funcionais

Quanto o sistema pode gastar e o que ele garante, com número e meta. Mudou algo que mexe numa meta? Meça antes e depois, e escreva no PR.

**Situação de cada meta:**
- **Conferida:** a CI ou um teste confere a cada mudança.
- **Medida:** há um número, mas ninguém confere sozinho.
- **Combinada:** a meta está decidida, mas ainda não é atingida nem conferida.
- **Sem medida:** ainda não foi medida.

## Eficiência

As metas de memória e de disco valem **para o Go no fim** (#86): antes disso, o .NET continua junto, e os números só pioram ou ficam iguais.

| Atributo | Medida | Hoje | Meta | Como medir | Situação |
|---|---|---|---|---|---|
| Memória da API em uso | MB do processo, ocioso e no pico de uma sincronização | ~225 MB (21/10/2026, 2.23, Windows, `.msi`) | Até 60 MB | Método da #86: 5 minutos depois de subir, sem requisições, e durante uma sincronização, com 3 leituras | Combinada em 31/10/2026. O topo do palpite do épico #70 |
| Tamanho da instalação | MB e arquivos de `%ProgramFiles%\FinancasLocal` e de `publicado/`, sem os dados | 157 MB (mesma medição) | Até 40 MB | Método da #86 | Combinada em 31/10/2026. O topo da estimativa do épico #70 |
| Tempo de partida | Segundos do início da API até `GET /api/auth/estado` responder | 2,2 s (31/10/2026, 2.50.31, Windows) | Até 5 s | `medir-desempenho.py`, 3 partidas | Medida |
| Tempo de resposta das telas principais | A chamada mais lenta ao abrir a tela | Painel 84 ms, Transações 23 ms, Investimentos 43 ms (mesma medição) | Até 300 ms por tela | `medir-desempenho.py`, com 5 anos de transações fictícias | Medida |

Partida e telas saem de `scripts/geral/medir-desempenho.py`, com 9.170 transações fictícias, num computador de desenvolvimento (Core i5, 16 GB):
- **É o tempo da API:** o navegador ainda desenha a tela.
- **Em cada tela vale a chamada mais lenta,** porque o navegador as faz em paralelo.
- **A saída para a internet é recusada na hora,** para o número não virar a espera de uma conexão.
- **Compare antes e depois na mesma máquina:** outro computador dá outro número.

As metas de partida e de telas dão de 2 a 4 vezes de folga sobre o medido, para caber um computador mais lento.

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
| Sem internet | O que continua funcionando | O site abre e mostra o que está no banco. Câmbio guardado continua valendo. Sincronização e cotações param até a internet voltar. Investimentos abre, mas cada abertura espera a falha do Banco Central (#534) | À mão | Sem medida. Investimentos: ~4,1 s por abertura com a saída numa porta fechada, no Windows |
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
