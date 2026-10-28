# Registros de decisão de arquitetura (ADRs)

Cada ADR registra **uma** decisão técnica que afeta a forma do sistema: o problema, as alternativas consideradas, o que foi escolhido e o custo dessa escolha. O objetivo é que quem chegar depois (inclusive o Claude Code) entenda **por que** o código é como é antes de mudar.

## Quando escrever uma ADR

Escreva (ou atualize) uma ADR quando a mudança:

- adiciona ou troca uma **dependência, biblioteca ou integração externa** (nova API, novo domínio de saída de rede);
- muda o **banco** além de uma tabela/coluna rotineira (motor, estratégia de esquema, cache, forma de guardar um tipo de dado);
- muda **segurança**: autenticação, segredos, CSP, exposição de rede, permissões;
- muda **infraestrutura** ou publicação: hospedagem, serviço do sistema, scripts, CI;
- estabelece ou quebra um **padrão de código** que vale para o projeto todo (camadas, nomes, forma de chamar a API).

Não precisa de ADR: correção de bug, refatoração local, ajuste visual, nova tela ou endpoint que só segue padrões já registrados.

## Como escrever

1. Copie [0000-modelo.md](0000-modelo.md) para `NNNN-titulo-em-kebab-case.md`, com o próximo número livre.
2. Preencha contexto, alternativas, decisão e consequências — **as consequências negativas são obrigatórias**.
3. Adicione a linha no índice abaixo.
4. Inclua a ADR no mesmo pull request da mudança que ela justifica.

## Ciclo de vida

| Status | Significado |
|---|---|
| Proposta | Em discussão (pull request aberto). |
| Aceita | Vale para o código atual. |
| Substituída por NNNN | Trocada por outra decisão. A ADR antiga **não é apagada**: muda o status e aponta para a nova. |
| Descontinuada | Deixou de valer sem substituta (ex.: a funcionalidade foi removida). |

Uma ADR aceita só é **editada** para correções, novos detalhes da mesma decisão ou atualização de consequências. Se a decisão em si mudou, crie uma ADR nova e marque a antiga como substituída.

As ADRs 0002 a 0006 e 0014 a 0021 foram escritas depois das decisões, a partir do código, da documentação e dos pull requests que as introduziram (citados em cada uma). O contexto delas é reconstruído; o que não estava registrado está marcado como *inferido*.

## Índice

| ADR | Título | Status |
|---|---|---|
| [0001](0001-registrar-decisoes-em-adrs.md) | Registrar decisões de arquitetura em ADRs | Aceita |
| [0002](0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md) | SQLite e Kestrel no lugar de SQL Server Express e IIS | Aceita (em parte substituída pela 0012; o Kestrel, depois, pela 0049) |
| [0003](0003-esquema-por-scripts-sql-idempotentes.md) | Esquema do banco por scripts SQL idempotentes, sem migrations | Aceita |
| [0004](0004-front-end-publicado-no-wwwroot-da-api.md) | Front-end publicado dentro da API (mesma origem) | Aceita |
| [0005](0005-segredos-criptografados-com-data-protection.md) | Segredos criptografados no banco com ASP.NET Data Protection | Aceita (vai ser substituída pela 0049 na troca dos segredos) |
| [0006](0006-cliente-http-proprio-para-a-pluggy.md) | Cliente HTTP próprio para a Pluggy, sem SDK | Aceita |
| [0007](0007-cotacao-de-cambio-pela-ptax-do-banco-central.md) | Cotação de câmbio pela PTAX do Banco Central, guardada para uso offline | Aceita |
| [0008](0008-moeda-estrangeira-somente-em-investimentos.md) | Moeda estrangeira só em investimentos; demais registros em reais | Aceita (em parte substituída pela 0009) |
| [0009](0009-ativos-no-exterior-na-carteira-manual.md) | Ativos no exterior na carteira manual | Aceita |
| [0010](0010-reservas-estimadas-pelo-extrato.md) | Reservas estimadas pelo extrato, com conferência manual | Substituída por 0037 |
| [0011](0011-ferramentas-de-instalacao-portateis-sem-python.md) | Ferramentas de instalação portáteis, sem depender de Python | Aceita (com o Go portátil desde a #231; o SDK do .NET sai depois, pelo Go: 0049) |
| [0012](0012-instalador-do-windows-em-powershell.md) | Instalador do Windows em PowerShell, sem bash | Aceita (em parte substituída pela 0029; o SDK do .NET, depois, pelo Go: 0049) |
| [0013](0013-testes-do-front-end-com-jest.md) | Testes automatizados do front-end com Jest | Aceita |
| [0014](0014-emprestimos-e-financiamentos-pelo-open-finance.md) | Empréstimos e financiamentos pelo Open Finance, separados e fora das despesas | Aceita |
| [0015](0015-parametros-do-sistema-no-banco-editaveis-pelo-site.md) | Parâmetros do sistema no banco, editáveis pelo site | Aceita |
| [0016](0016-icones-do-lucide-empacotados-no-build.md) | Ícones do Lucide empacotados no build | Aceita |
| [0017](0017-preferencias-de-exibicao-no-navegador.md) | Preferências de exibição ficam no navegador | Aceita |
| [0018](0018-estado-da-tela-na-url-e-detalhe-por-navegacao.md) | Estado da tela na URL e detalhe por navegação | Aceita |
| [0019](0019-status-da-sincronizacao-por-consulta-periodica.md) | Status da sincronização por consulta periódica a um endpoint leve | Aceita |
| [0020](0020-glossario-unico-com-balao-na-top-layer.md) | Glossário único, com o balão na top layer (Popover API) | Aceita |
| [0021](0021-componentes-padrao-para-graficos-e-telas-densas.md) | Componentes padrão para gráficos e telas densas | Aceita |
| [0022](0022-erros-do-sistema-gravados-no-banco.md) | Erros do sistema gravados no banco e vistos no site | Aceita |
| [0023](0023-edicao-em-massa-no-modo-edicao-de-transacoes.md) | Edição de categorias em massa como "modo edição" de Transações, com desfazer guardado na API | Aceita |
| [0024](0024-banco-de-cada-conta-com-selo-local.md) | Banco de cada conta e cartão, com selo desenhado no próprio site | Aceita |
| [0025](0025-tipo-de-lancamento-pelo-open-finance-e-descricao.md) | Tipo de lançamento: dado do Open Finance, com a descrição como reserva | Aceita |
| [0026](0026-manual-do-site-como-dados-no-front-end.md) | Manual do site como dados no front-end, com testes que o mantêm em dia | Aceita |
| [0027](0027-conferencia-de-investimentos-com-previa-e-desfazer.md) | Conferência de investimentos: correções sugeridas pelo extrato, aplicadas só com confirmação | Aceita (em parte substituída pela 0044) |
| [0028](0028-gerenciador-do-windows-e-atualizacao-pelo-github.md) | Gerenciador do Windows, com atualização pelo GitHub Releases | Aceita (em parte substituída pela 0040) |
| [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md) | Instalador .msi para Windows, com WiX, gerado e testado pela CI | Aceita (em parte substituída pela 0039) |
| [0030](0030-regras-de-descricao-em-tabela-propria.md) | Regras de descrição em tabela própria, com o nome do banco sempre guardado | Aceita |
| [0031](0031-transferencia-fora-dos-relatorios-so-entre-contas-suas.md) | Transferência só fica fora dos relatórios quando é entre contas suas | Aceita |
| [0032](0032-controles-padrao-interruptor-e-campo-de-valor.md) | Controles padrão: interruptor, caixa de marcar, campo de valor, campo de data e lista suspensa | Aceita |
| [0033](0033-gerenciador-no-linux-com-avalonia.md) | Gerenciador no Linux, com Avalonia e núcleo comum ao do Windows | Aceita (em parte substituída pela 0040) |
| [0034](0034-sem-sri-no-build-do-front-end.md) | Build do front-end sem SRI, para manter a CSP sem script inline | Aceita |
| [0035](0035-plano-das-metas-pelo-caixa-calculado-na-tela.md) | Plano das metas pelo caixa do mês (fatura que vence entra), com a divisão calculada na tela | Aceita |
| [0036](0036-receita-e-despesa-pelo-tipo-estorno-abate-gasto.md) | Receita e despesa pelo tipo, não pelo sinal; estorno no cartão abate o gasto | Aceita |
| [0037](0037-reservas-pelo-valor-do-open-finance.md) | Reservas pelo valor que o Open Finance informa; a estimativa pelo extrato fica para o resto | Aceita |
| [0038](0038-meli-dolar-como-reserva-em-dolar.md) | Méli Dólar como reserva em dólar, estimada pelo extrato e convertida pela PTAX | Aceita |
| [0039](0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md) | A versão sobe a cada pull request, pelo tipo do commit | Aceita |
| [0040](0040-atualizar-pelo-develop-so-na-instalacao-de-desenvolvimento.md) | Atualizar pelo develop, pelo gerenciador, só na instalação de desenvolvimento | Aceita (em parte substituída pela 0053) |
| [0041](0041-testes-no-navegador-com-playwright.md) | Testes no navegador com Playwright, com a CSP da API e uma API falsa | Aceita |
| [0042](0042-contas-fora-do-saldo-continuam-nos-relatorios.md) | Contas fora do saldo e arquivadas continuam nos relatórios | Aceita |
| [0043](0043-assinaturas-e-recorrentes-em-contas-a-pagar.md) | Assinaturas e cobranças recorrentes estendendo Contas a pagar | Aceita |
| [0044](0044-trocos-sem-duvida-classificados-na-sincronizacao.md) | Trocos "sem dúvida" classificados na sincronização, com aviso e "Desfazer" | Aceita |
| [0045](0045-historico-de-saldo-dos-investimentos-do-open-finance.md) | Histórico de saldo dos investimentos do Open Finance, um registro por dia | Aceita |
| [0046](0046-menu-lateral-em-grupos-que-abrem-e-fecham.md) | Menu lateral em grupos que abrem e fecham (sanfona), vários abertos ao mesmo tempo | Aceita |
| [0047](0047-ci-paralela-so-testes-afetados-no-pr.md) | CI em jobs paralelos e, no PR, só os testes afetados; .msi só quando o PR mexe nele | Aceita |
| [0048](0048-cotacao-de-criptomoedas-pelo-mercado-bitcoin.md) | Cotação de criptomoedas pela API pública do Mercado Bitcoin | Aceita |
| [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md) | Back-end em Go, trocado aos poucos atrás do .NET (strangler fig), com sqlc, modernc.org/sqlite e shopspring/decimal; o .NET sobe o Go como processo filho e repassa os prefixos de `Go:Prefixos` (#231) | Aceita |
| [0050](0050-msi-gerado-no-merge-e-nao-no-pr.md) | O .msi é gerado no merge para o develop (e na tag), e não no PR | Aceita |
| [0051](0051-testes-do-back-end-em-go-cada-regra-com-teste.md) | Testes do back-end em Go: cada regra com teste unitário, além do contrato; SQLite real, `httptest` e só a biblioteca padrão; teste de mutação semanal com o gremlins (#373) | Aceita |
| [0052](0052-gerenciador-em-go-com-bandeja-e-pagina-local.md) | Gerenciador em Go: ícone na bandeja e janela própria com o Wails (#276) | Aceita |
| [0053](0053-receber-versoes-de-desenvolvimento-por-escolha.md) | Receber versões de desenvolvimento por escolha, no gerenciador em Go | Aceita |
