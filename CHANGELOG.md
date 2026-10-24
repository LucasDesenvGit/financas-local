# Changelog

Formato baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/) e [versionamento semântico](https://semver.org/lang/pt-BR/).

## [2.41.0] — 2026-10-23 · desenvolvimento

### Funcionalidades
- **Configurações › Conexões: o selo do banco no lugar da bolinha colorida** (#376). Cada conexão do Open Finance aparece com o selo do banco, o mesmo de Contas e cartões, achado pelo nome da instituição; banco que o site não conhece aparece com o ícone neutro de banco. O número de contas e o conector ("via MeuPluggy") ficam numa linha embaixo do nome. Os selos vão no próprio site: nada é baixado da internet.

## [2.40.2] — 2026-10-23 · desenvolvimento

### Publicação e documentação
- **Go: relógio de teste e contrato rodando contra o Go atrás do .NET** (#363, épico #70). Nada muda para quem usa. Para quem desenvolve:
  - o Go ganha uma opção só para teste, `-inicio-do-relogio`, desligada por padrão: o relógio começa num instante dado e anda dali, como a libfaketime faz com o .NET no contrato (ela não age sobre o Go). O .NET a passa ao Go quando `Go:InicioDoRelogio` tem um instante (vazio no `appsettings.json`);
  - o contrato compila o Go e o sobe atrás do .NET, com o mesmo relógio. `PREFIXOS_GO="/api/..."` passa um assunto pelo Go com os mesmos esperados; `--go` passa os assuntos ligados em `testes/contrato/assuntos-no-go.txt`, e a CI roda as duas rodadas. O `/api/go/saude` devolve a hora e a data do relógio do Go e ganhou casos de contrato (58 casos);
  - para os assuntos feitos em paralelo (#75, #77, #81) não se chocarem: cada assunto registra as rotas com uma linha no `internal/api`, tem um número de arquivo de casos reservado (tabela no README do contrato) e uma entrada própria em `Go:Prefixos`, que passa a ser uma entrada por assunto (o texto com `;` continua valendo).

## [2.40.1] — 2026-10-23 · desenvolvimento

### Correções
- **Investimentos: Câmbio e Criptomoedas com as colunas alinhadas** (#377). Os dois blocos usam a mesma grade:
  - código com largura fixa, bandeira ou ícone, nome, valor, variação e fonte;
  - o nome começa e o valor termina no mesmo ponto em todas as linhas, e também de um bloco para o outro;
  - no câmbio, que não tem variação, a data termina onde termina a de cripto;
  - no celular, a variação e a fonte ficam embaixo do nome.

## [2.40.0] — 2026-10-23 · desenvolvimento

### Funcionalidades
- **Categorias com o ícone na cor delas, no lugar da bolinha** (#336).
  - **Onde aparece:** no Painel ("Gastos por categoria" e Orçamentos), nas Análises (Relatórios, Visão anual e Projeção), em Categorias e regras e em Orçamentos, inclusive na sugestão de orçamentos.
  - **Como é:** um quadrado arredondado com o ícone da categoria (talheres em Alimentação, casa em Moradia…), na cor dela, com o fundo dessa cor suave. Categoria sem ícone conhecido mostra a primeira letra do nome.
  - **Contraste:** o ícone tem pelo menos 3:1 sobre o fundo nos dois temas e em todas as paletas, conferido pelo teste.
  - **A rosca do Painel:**
    - o total do mês fica no centro;
    - as categorias menores que 3% ficam juntas em "Outras", e clicar nessa fatia abre todas as despesas do mês;
    - as fatias ganham um espaço entre elas.
  - **A lista do Painel:** cada categoria mostra o ícone, uma barrinha embaixo do nome, o valor e o %. Aparecem as 8 maiores, e "Mostrar todas" abre o resto.
  - **Orçamentos no Painel:** o bloco ocupa a altura do vizinho, com as linhas dividindo a altura por igual e, no rodapé, o total gasto, o total orçado e quantos passaram do limite.
  - **Animações:** as categorias que "Mostrar todas" acrescenta entram crescendo, uma depois da outra, e o bloco de Orçamentos acompanha. O conteúdo de "Mais análises", em todas as telas, desce aparecendo ao abrir e sobe sumindo ao fechar. Com "reduzir movimento" ligado no sistema, não há animação.

## [2.39.1] — 2026-10-23 · desenvolvimento

### Correções
- **Contas a pagar: nome, conta e forma de cobrança alinhados** (#379). O selo da cobrança fica numa coluna própria. À direita dele, o nome, a conta (com o selo do banco), a forma de cobrança ("paga à mão", "débito automático"…) e a observação ficam empilhados, todos começando no mesmo ponto. Antes, as linhas de baixo começavam antes do nome, cada uma num lugar diferente, o que ficava mais visível nas transferências. Vale também para as sugestões achadas no extrato.

## [2.39.0] — 2026-10-23 · desenvolvimento

### Funcionalidades
- **Metas: a data alvo pelo valor que você vai guardar por mês** (#359). No formulário, "Quanto vou guardar por mês" calcula quando a meta chega ao alvo, contando o mês atual, e preenche a data alvo. Uma frase mostra a conta ("Com R$ 1.000,00 por mês, você chega lá em mar./2027 (6 meses)"). Mudar o alvo ou o que já está guardado recalcula; a data continua editável à mão. Criar sem data alvo continua possível, e a meta fica "sem data definida". O valor por mês fica gravado na meta (coluna nova `Meta.GuardarPorMes`) e volta ao editar. O plano guarda esse valor todo mês, antes das outras metas e pela prioridade, sem passar da parte da sobra que vai para as metas (a meta avisa o mês em que não dá). No formulário, valor por mês acima do que sobra para as metas (a maior sobra dos próximos 12 meses, menos o que as outras metas já guardam) é bloqueado: o erro aparece ao digitar, com o máximo, e "Salvar" fica desabilitado. O plano mês a mês para no mês em que a meta é alcançada, e os degraus também não passam dessa parte da sobra. No topo, a recomendação vira uma caixa em destaque numa linha só, com o limite ao lado e o essencial em negrito; os avisos amarelos das metas (prazo e valor por mês) viram caixas no mesmo estilo, com valores e meses em negrito, e os links do aviso de prazo ficam na linha de baixo, separados por "|". O ícone da meta aparece, translúcido, no centro do anel do percentual, e o miolo do anel enche de baixo para cima na cor da meta, até o percentual guardado. Os botões do cartão ficam "+ Aporte" e "Editar", e a etiqueta "Concluída" (ou "Concluída por você"), com maiúscula. A meta concluída ganha uma caixa verde "Meta alcançada!" no lado do plano, e o hover do título dela fica verde translúcido. O gráfico "Como está indo" acompanha os aportes: todos no mesmo dia, um ponto por aporte na hora e minuto (a API devolve a hora do aporte, em Brasília); todos no mesmo mês, um ponto por dia; em meses diferentes, mês a mês. A lista de aportes mostra a hora.

## [2.38.1] — 2026-10-23 · desenvolvimento

### Correções
- **Metas: card espremido com uma meta só, e um card mais fácil de ler** (#335).
  - **Cada meta é uma sanfona** na largura toda, como os grupos de "Assinaturas e recorrentes". Fechada, mostra o percentual, os valores e quanto guardar no mês. Todas começam abertas, as fechadas ficam lembradas neste navegador, e há "Fechar todas"/"Abrir todas".
  - **Aberta, no computador**, o anel, os valores e a prioridade ficam centralizados à esquerda, e o plano do mês à direita; as ações ("+ aporte", "editar") ficam embaixo, na largura toda. No celular, um embaixo do outro.
  - **"Consigo guardar outro valor"** fica na mesma linha de "Este mês: guarde".
  - **A lista "Depois:" saiu.** O "Plano mês a mês" já vem aberto, com 5 meses; "Mostrar mais" acrescenta 5 de cada vez, até 12.
  - **Meta sem data alvo** mostra "sem data definida", e as metas não desalinham.

## [2.38.0] — 2026-10-23 · desenvolvimento

### Funcionalidades
- **O back-end em Go passa a rodar no site, atrás do .NET** (#231, épico #70, ADR 0049). Nada muda nas telas; é a base para cada assunto passar para o Go, um de cada vez:
  - o .NET continua cuidando do login, do CSRF e dos segredos, inicia o Go junto com ele e repassa ao Go só as rotas da lista `Go:Prefixos` (por enquanto, só `/api/go/saude`, que confere o caminho todo até o banco). Tirar uma rota da lista e reiniciar o serviço a devolve ao .NET, sem reinstalar;
  - o Go só responde ao .NET (segredo interno, só no próprio computador), para junto com ele e, se cair, volta sozinho;
  - o banco passa para o modo WAL, para os dois usarem o mesmo arquivo ao mesmo tempo. Para restaurar uma cópia, com o serviço parado, apague também os arquivos `-wal` e `-shm` ao lado do banco;
  - a instalação pelos scripts baixa o Go (conferido pelo SHA-256 oficial) e compila o back-end em Go; ela fica mais demorada e ocupa mais espaço em `ferramentas/` enquanto o .NET e o Go convivem. O `.msi` só ganha o executável do Go.

## [2.37.9] — 2026-10-23 · desenvolvimento

### Publicação e documentação
- **Go: testes de contrato do back-end** (#72, épico #70). Primeiros testes automatizados do back-end; nada muda para quem usa. Para quem desenvolve:
  - o que a API .NET responde e grava hoje fica registrado, caso a caso: status, JSON e as linhas que mudam no banco, com o formato exato de cada coluna. O Go vai ter de responder e gravar igual;
  - a API roda sobre um banco de dados inventado, sem internet e com o relógio controlado, para o resultado ser o mesmo a cada execução;
  - já cobertos: entrar, sair, configurar o usuário, trocar a senha, bloqueio por tentativas, auditoria e a Parametrização (54 casos). Cada próxima etapa grava os casos do próprio assunto;
  - a CI roda o contrato em todo PR que mexe no back-end.

## [2.37.8] — 2026-10-23 · desenvolvimento

### Publicação e documentação
- **Go: esquema do banco ao iniciar, igual ao do .NET** (#297, épico #70). Ainda não roda no site: até a inversão (#85), quem cria e atualiza o banco continua sendo o .NET. Para quem desenvolve:
  - o Go aplica os mesmos scripts de `banco/` e as mesmas colunas novas do `2-criar-banco.sh`, com a mesma assinatura no banco: trocar entre o .NET e o Go não refaz a cópia de segurança a cada partida;
  - cópia em `dados/backups` antes de mudar um banco que já existe (as 5 mais recentes) e dados iniciais só no banco novo, como no .NET;
  - conferido contra o código do .NET: banco igual tabela por tabela, coluna por coluna, índices e dados iniciais;
  - mudou o `banco/`? Rode `go generate ./...` em `src/backend-go` (passo novo no `docs/BANCO-DE-DADOS.md`).

## [2.37.7] — 2026-10-23 · desenvolvimento

### Publicação e documentação
- **CI: job do back-end em Go** (#231, épico #70). Roda em paralelo com os outros, só quando o PR mexe no que o Go lê, e entra no check `build`:
  - formatação, `go vet`, testes com o detector de corrida, código gerado pelo `sqlc` em dia, compilação para Windows e `govulncheck`;
  - o Dependabot passa a acompanhar os módulos do Go.

## [2.37.6] — 2026-10-23 · desenvolvimento

### Publicação e documentação
- **Go: esqueleto do servidor** (#73, épico #70). Ainda não roda no site. Para quem desenvolve:
  - o servidor em Go serve o site em HTTPS, só no próprio computador, lendo os mesmos `appsettings` e o mesmo certificado que os scripts e o `.msi` já geram;
  - cabeçalhos de segurança, cache, erros e o filtro de endereço iguais aos do .NET, conferidos contra o site rodando, rota por rota;
  - toda chamada a `/api` ainda responde 404: as rotas entram assunto por assunto.

## [2.37.5] — 2026-10-23 · desenvolvimento

### Publicação e documentação
- **Go: camada de banco compatível com o EF Core** (#74, épico #70). Primeiro código Go do projeto, em `src/backend-go/`; ainda não roda no site. Para quem desenvolve:
  - tipos `Decimal`, `Data`, `DataHora` e `Guid`, que gravam e leem o banco no mesmo formato do .NET, conferidos contra um gabarito gerado pelo próprio EF Core;
  - abertura do banco com os mesmos ajustes do .NET, transação que desfaz sozinha em caso de erro, e o relógio no fuso de Brasília;
  - `sqlc` lendo o esquema do `banco/` (com as colunas novas do `2-criar-banco.sh`) e as primeiras consultas, da `Configuracao`;
  - lida uma cópia do banco real inteira sem nenhum erro de formato.

## [2.37.4] — 2026-10-23 · desenvolvimento

### Publicação e documentação
- **ADR 0049: o back-end vai para Go, trocado aos poucos atrás do .NET** (#71, épico #70). Nada muda no site ainda; a ADR registra como a migração vai ser feita:
  - o .NET continua na frente (login, segredos, esquema do banco) e repassa ao Go, um assunto de cada vez, as rotas que já foram migradas; voltar um assunto é tirar a rota da lista;
  - o C# de cada assunto é apagado depois de uma versão publicada com ele no Go;
  - o esquema novo dos segredos é feito em Go, mas só é usado na troca; nesse dia, o Client Secret da Pluggy e o token da brapi são informados de novo, uma vez;
  - bibliotecas: `net/http`, `modernc.org/sqlite`, `sqlc`, `shopspring/decimal` e `log/slog`;
  - as ADRs 0002, 0005, 0011 e 0012 apontam o que a 0049 vai substituir, e o `CLAUDE.md` ganhou as convenções do código Go.

## [2.37.3] — 2026-10-22 · desenvolvimento

### Correções
- **Gerenciador (Windows): a seta dos botões de baixar e a barra de rolagem no tema escuro** (#337).
  - Em "Atualizar para a última do develop" e "Baixar o instalador .msi", a seta saía de um caractere da fonte de ícones do Windows, fina e com outro peso que o texto. Agora ela é desenhada: o ícone "download", o mesmo do site.
  - No tema escuro, a caixa com a saída da atualização e a janela "O que mudou" tinham a barra de rolagem clara do Windows. Agora ela fica escura, com o tema escuro do próprio Windows.

## [2.37.2] — 2026-10-22 · desenvolvimento

### Correções
- **Painel: "Últimas transações" mostrava parcelas futuras** (#334). As parcelas que o banco já manda com data futura (ex.: "Casasbahiacom 11/11" em 15/04) ficavam no topo e escondiam o que aconteceu nos últimos dias. Agora a lista vai até hoje, e as futuras ficam no botão "Futuras (N)", com o ano na data. "ver todas" abre Transações no mesmo recorte (até hoje, ou de amanhã em diante).

## [2.37.1] — 2026-10-22 · desenvolvimento

### Publicação e documentação
- **CI: o instalador `.msi` só é gerado no merge para o develop, e não mais no PR** (#344, ADR 0050). O PR é refeito várias vezes, e cada `.msi` gerado nele era jogado fora; o PR fica ~7 min mais rápido quando mexe no instalador. Para testar o instalador antes do merge, o workflow "Versão (instalador .msi)" roda à mão na branch. Saiu o `scripts/ci/precisa-msi.py`.

## [2.37.0] — 2026-10-22 · desenvolvimento

### Funcionalidades
- **Seletor de mês: o ano abre os anos** (#342). No quadro do seletor de mês (Painel, Contas e cartões, Orçamentos…), o ano é um botão que abre uma página de 12 anos, como o título do calendário dos campos de data. Escolher um ano volta aos meses dele, e Esc volta um nível antes de fechar. A regra entrou no `CLAUDE.md` e num teste que confere os dois quadros, para nenhum componente de data novo ficar de fora.

## [2.36.1] — 2026-10-22 · desenvolvimento

### Correções
- **Investimentos › Mais análises: "Distribuição" sem o canto vazio** (#330). Sem ações nem FIIs em carteira, não há o card de rentabilidade × Selic, e a distribuição ficava em meia largura, com a outra metade vazia. Agora ela ocupa a largura toda, com a rosca à esquerda e a lista à direita (no celular, um embaixo do outro).

## [2.36.0] — 2026-10-22 · desenvolvimento

### Funcionalidades
- **Criptomoedas: a fonte da cotação em cada moeda** (#329). Cada linha diz de onde veio o preço e quando ("Mercado Bitcoin · 22/10 16:00"), como o "PTAX de …" do câmbio. Com mais de 2 horas, a linha diz "desatualizada". No celular, a fonte e a hora ficam numa linha própria, e a variação em 24 h não fica mais espremida.

## [2.35.0] — 2026-10-22 · desenvolvimento

### Funcionalidades
- **Contas e cartões: as faturas de outros meses** (#323). O seletor de mês, como no Painel, fica em "Cartões de crédito" e muda a fatura de cada cartão e o total "Faturas de …". O mês fica na URL. Saldo, dívida total, limite e "Fatura em aberto" continuam sendo os de hoje, e a tela diz isso. O link "Faturas do mês" do Painel abre Contas no mesmo mês. Endpoint novo: `GET /api/contas/faturas-do-mes`.

## [2.34.0] — 2026-10-22 · desenvolvimento

### Funcionalidades
- **Notificações na barra do topo** (#327). Os orçamentos estourados e os avisos (conta vencida, Open Finance, erros…) saíram do topo do Painel e foram para o **sino** da barra do topo, em qualquer tela.
  - O sino mostra quantas notificações estão pendentes; o que foi ocultado ou minimizado sai da conta.
  - O card abre pelo sino e fecha com Esc, com um clique fora ou ao ir para outra tela.
  - Os cards e o "Ocultar"/"Mostrar" são os mesmos de antes, e as escolhas continuam guardadas no navegador.
  - O Painel não é mais empurrado para baixo pelos avisos.
  - Se as notificações não carregarem, o card diz isso e oferece "Tentar de novo".
  - Endpoint novo `GET /api/painel/avisos` (os avisos do mês atual, sem o resto do Painel).

## [2.33.0] — 2026-10-22 · desenvolvimento

### Funcionalidades
- **Selo da transação também pelas etiquetas** (#325). Antes, o selo vinha só do nome. Agora:
  - "Pix enviado" e "Pix recebido" sem serviço conhecido ganham o selo do Pix (ex.: "Transferência Recebida|Fulano");
  - "Pagamento de fatura" ganha o logo do banco citado no texto (ex.: "Fatura Paga Itau Uniclass" → Itaú);
  - sem marca nem categoria, aparece o ícone do tipo, o mesmo da etiqueta, no lugar do recibo;
  - o **Keeta** (delivery) entrou na lista de serviços.

## [2.32.0] — 2026-10-22 · desenvolvimento

### Funcionalidades
- **Fatura em aberto no Painel e em Contas e cartões** (#322). Em Contas e cartões, cada cartão ganhou o interruptor "Fatura no Painel". O card "Fatura em aberto", no Painel ("Situação hoje") e no topo de Contas e cartões, soma a fatura em aberto dos cartões ligados: a do mês, enquanto não fecha; depois, a próxima. "ver detalhes" mostra a de cada cartão, com quando fecha, quando vence e o link para o histórico. A escolha fica no navegador, como as outras preferências de exibição (ADR 0017).

## [2.31.0] — 2026-10-21 · desenvolvimento

### Funcionalidades
- **Metas: gráfico "Como está indo"** (#318). Em cada meta, um botão abre o gráfico com o total guardado no fim de cada mês, o caminho do plano até o valor alvo e o próprio alvo. Cada "+ aporte" muda o gráfico, e embaixo ficam os últimos aportes. **Banco:** tabela nova `MetaAporte`, criada sozinha ao atualizar. Ela guarda a data e o valor de cada aporte; antes, o aporte só somava no total. O histórico começa a partir desta versão.

## [2.30.0] — 2026-10-21 · desenvolvimento

### Funcionalidades
- **Ícone em cada meta** (#316): casa, carro, viagem, estudo, reserva e outros 11, escolhidos no formulário e mostrados no cartão, na cor da meta. Ajuda a achar a meta de olho. O cartão ganhou divisórias entre o progresso, o plano do mês e as ações. **Banco:** coluna nova `Meta.Icone`, criada sozinha ao atualizar.

### Correções
- **Metas: "concluída" com 1% e "guardar R$ 0,00"** (#316).
  - Quando a meta chega ao alvo, ela fica concluída. Se depois o alvo aumentava, a meta continuava concluída (ex.: R$ 200 de R$ 30.000). Agora, aumentar o alvo para acima do que já foi guardado devolve a meta ao plano, e o formulário diz isso.
  - Marcada como concluída antes de chegar ao alvo, o cartão diz "concluída por você", e o formulário mostra quanto falta.
  - Sem meta em andamento, o assistente diz isso, em vez de "recomendo guardar R$ 0,00".
  - "R$ 200,00de R$ 30.000,00" ganhou o espaço.
  - As cores do formulário têm nome para o leitor de tela ("Cor azul", e não o código).

## [2.29.0] — 2026-10-21 · desenvolvimento

### Funcionalidades
- **Projeção mais fácil de ler** (#314).
  - **O mês em que o saldo vira** ganha um selo, "saldo volta ao positivo" ou "saldo fica negativo", e a frase do gráfico diz o mês (sem valores, com "ocultar valores").
  - **"Gasto médio por categoria" na largura toda**, em duas colunas no computador, sem a metade da direita vazia.
  - **A porcentagem diz o que compara:** "último mês +34%", com a regra das cores escrita embaixo (antes só num texto que aparecia ao passar o mouse).
  - **O nome da categoria abre Transações** com as despesas dela nos mesmos meses da média, e cada uma tem a barrinha proporcional, como em Relatórios.

## [2.28.1] — 2026-10-21 · desenvolvimento

### Correções
- **Projeção › Mês a mês: "Saldo previsto" cortado em tela de notebook** (#312). A 1366 px, a tabela ficava em meia largura, ao lado de "Gasto médio por categoria", e a última coluna só aparecia rolando para o lado; "em parcelas" quebrava em duas linhas. Agora a tabela ocupa a largura toda (o gasto por categoria vai para baixo), e "em parcelas" fica numa linha só.

## [2.28.0] — 2026-10-21 · desenvolvimento

### Funcionalidades
- **Instalação mais clara quando algo atrapalha** (#300).
  - **Porta 5443 ocupada:** a instalação diz qual programa está nela e o PID, em vez de o site simplesmente não subir.
    - Pelo `.msi`, a instalação para com a mensagem no log do instalador.
    - Pelo `instalar-windows.cmd`, ela para antes de trocar a versão, sem mudar nada.
    - Pelos scripts bash, ela avisa antes de ligar o serviço e repete no teste final, se o site não responder.
  - **Firefox:** no fim da instalação, se ele estiver instalado, aparece como fazê-lo aceitar o certificado do site.
  - **Certificado no Linux:** quando a confiança automática falha, a instalação salva a parte pública do certificado (sem a chave) e mostra os comandos da sua distribuição (Debian/Ubuntu, Fedora, Arch) e do Chrome/Chromium.
  - **Gerenciador no Linux:** o `instalar.sh` já instala o gerenciador no fim (`--sem-gerenciador` para não instalar). No Windows pelos scripts, ele mostra o comando.

## [2.27.4] — 2026-10-21 · desenvolvimento

### Publicação e documentação
- **A CI liga a API de verdade em todo PR de back-end** (#309). Antes, a API só era ligada no teste do instalador do Windows, que roda em poucos PRs: um erro que só aparece ao iniciar (serviço sem registro, configuração, esquema do banco) passava batido. Agora o job "API e banco" liga a API num banco temporário e espera ela responder (`scripts/ci/ligar-api.sh`, ~10 s).

## [2.27.3] — 2026-10-21 · desenvolvimento

### Publicação e documentação
- **PR só de CI não roda o instalador do Windows** (#307). PR com título `ci:` não gera nem testa o `.msi` (~7 min), mesmo mexendo no `versao.yml`. Se ele mexer de fato no instalador, no gerenciador ou na inicialização da API, o `.msi` roda, e a CI avisa que o título não combina. Registrado no `CLAUDE.md` e na ADR 0047.

## [2.27.2] — 2026-10-21 · desenvolvimento

### Correções
- **O site abre mais rápido: cache longo em todos os arquivos do build** (#305). Os arquivos com hash no nome (que nunca mudam de conteúdo) ficam um ano no cache do navegador, mas a regra só aceitava maiúsculas e números, e o hash do Angular de hoje tem minúsculas, "_" e "-": só 3 dos 35 arquivos recebiam o cache longo, e o navegador perguntava de novo pelos outros 32 a cada abertura. Agora os 35 recebem; `index.html`, favicon, manifesto e ícones continuam sendo conferidos a cada abertura. A CI confere a regra contra nomes de um build de verdade (`scripts/ci/verificar-cache-estaticos.py`).

## [2.27.1] — 2026-10-21 · desenvolvimento

### Publicação e documentação
- **Skill e agente do Claude Code para a migração do back-end para Go** (épico #70). A skill `migrar-para-go` e o agente `dev-go` analisam o C# de um assunto e o portam para Go seguindo as etapas do épico:
  - contrato idêntico ao do .NET, banco no formato do EF Core, dinheiro nunca em `float64`;
  - nenhum código Go antes da ADR da #71; troca aos poucos, com o .NET na frente repassando ao Go os assuntos migrados;
  - as armadilhas C# → Go: arredondamento bancário do `Math.Round`, lista vazia que vira `null`, ordenação estável, busca com acento.
- O CHANGELOG da 2.17.0 que estava só no `main` (#249) já tinha voltado com a #294; o `main` entra no histórico do `develop` sem mudar o texto.

## [2.27.0] — 2026-10-21 · desenvolvimento

### Funcionalidades
- **Gerenciador: "O que mudou" legível e só com o que é novo para você** (#294).
  - As notas aparecem formatadas (títulos, listas, negrito e links clicáveis), e não mais com os símbolos do Markdown.
  - Primeiro vem só o que mudou desde a versão instalada; o resto (como instalar, versões anteriores e tudo desde a última publicada) fica em **Ver tudo**.
  - As notas da versão de desenvolvimento listam, por versão, os PRs que entraram nela.

### Publicação e documentação
- **CHANGELOG com uma seção por versão** (#294). O "Não publicado" juntava tudo desde a 2.1.0, inclusive o que já tinha saído na 2.17.0: a seção da 2.17.0 ficou só na branch da versão e não voltou para o develop. Agora:
  - a 2.17.0 voltou, com o texto publicado;
  - o que entrou depois está na versão de cada PR (de 2.17.1 a 2.27.0), achada pelo histórico do git, com "· desenvolvimento" até ser publicada;
  - cada versão tem um título de cada tipo (Funcionalidades, Correções…), e a CI confere (`scripts/ci/verificar-changelog.py`);
  - publicar uma versão é tirar o "· desenvolvimento" do título dela; as notas juntam as versões de desenvolvimento desde a última publicada.

## [2.26.1] — 2026-10-21 · desenvolvimento

### Correções
- **Gráficos de linha num padrão só** (#292). Na Visão anual, uns eram preenchidos e outros não, e o **Patrimônio estimado** era preenchido até o zero: com valor negativo, a faixa ficava acima da linha. Agora todos seguem a mesma regra, aplicada sozinha pelo componente de gráfico (ADR 0021), também em Investimentos e FIIs:
  - a linha principal é preenchida da linha até a base, com um ponto em cada mês;
  - a linha tracejada de comparação fica sem preenchimento.

## [2.26.0] — 2026-10-21 · desenvolvimento

### Funcionalidades
- **Gerenciador: o menu do botão direito na bandeja com o visual do site** (#289).
  - **No Windows:**
    - as cores do site, no tema claro e no escuro, como a janela;
    - um ícone em cada item e "Abrir o site" em destaque;
    - itens mais altos e cantos arredondados no Windows 11;
    - **no topo, a situação do site**, por exemplo "Finanças Local · No ar", com o ponto colorido.
  - **Teclado e alto contraste:** o teclado continua funcionando e pula a linha da situação. Com o alto contraste do Windows ligado, fica o desenho do sistema.
  - **No Linux:** o menu ganha a mesma linha da situação. As cores lá são do ambiente gráfico.

## [2.25.0] — 2026-10-21 · desenvolvimento

### Funcionalidades
- **Criptomoedas em Investimentos** (#279, ADR 0048): logo abaixo do Câmbio, o preço em reais do Bitcoin, do Ethereum, da Solana e do Tether.
  - É o último negócio no Mercado Bitcoin, que o site guarda sozinho a cada 30 minutos, 24 horas por dia, com a data e a hora sempre à vista.
  - Cada moeda tem o ícone da marca ao lado do código, um arquivo do próprio site (simple-icons, CC0).
  - A variação em 24 h é calculada pelo próprio site, contra a cotação guardada um dia antes. Aparece depois do primeiro dia com o site ligado; sem esse dado, a tela diz isso em vez de inventar um número.
  - **Banco:** tabela nova `CotacaoCripto`, criada sozinha ao atualizar.
  - **Rede:** domínio novo de saída, `api.mercadobitcoin.net`. Só os pares fixos (BTC-BRL…) são enviados, nenhum dado seu.

## [2.24.0] — 2026-10-21 · desenvolvimento

### Funcionalidades
- **Câmbio em Investimentos: bandeira de cada moeda e mais moedas** (#278).
  - Cada moeda ganhou a bandeira do país à direita do código (a do euro é a da União Europeia).
  - Além do dólar, do euro e do iene, "Mais 7 moedas" mostra as outras que o Banco Central publica na PTAX: libra, franco suíço, dólar canadense, dólar australiano e as coroas dinamarquesa, norueguesa e sueca. O site lembra se você deixou aberto.
  - As bandeiras são arquivos do próprio site ([flag-icons](https://github.com/lipis/flag-icons), MIT), sem buscar nada na internet.

## [2.23.4] — 2026-10-21 · desenvolvimento

### Publicação e documentação
- **Testes afetados também localmente** (#286): `npm run test:afetados`, em `src/frontend`. Ele roda o Jest, o build e os testes no navegador **só no que mudou** desde o develop, inclusive o que ainda não foi commitado, com a mesma escolha que a CI faz no PR. Numa mudança de uma tela, ~45 s, contra ~2,7 min da suíte inteira, que continua rodando na CI, no merge.

## [2.23.3] — 2026-10-21 · desenvolvimento

### Correções
- **Gráficos de barras com linha: a linha fica por cima, com um ponto em cada mês** (#282). Ela sumia atrás das barras no **Fluxo de caixa mensal** (Relatórios) e no **Saldo previsto** (Projeção). Agora é um padrão do site, aplicado sozinho a todo gráfico de barras com linha (também na Visão anual e em Investimentos), registrado na ADR 0021.
  - No Fluxo de caixa, a tabela ganhou a coluna "Saldo acumulado".
  - "Emergências" só aparece na legenda e no "como ler" quando houve alguma no período.
- **Visão anual › Resultado acumulado:** o preenchimento ia da linha até o zero. Nos meses com acumulado negativo, a faixa ficava acima da linha e parecia invertida; agora vai sempre da linha até a base do gráfico.
- **Campos de número sem as setinhas do navegador** (#283): elas não tinham o estilo do site e destoavam no tema escuro (Parametrização e outras telas). As setas do teclado continuam mudando o valor.
- **Configurações › Segurança › Acessos recentes** (#284):
  - a data com hora não fica mais por cima do evento (a mesma correção vale para as datas de "Maiores despesas" em Relatórios);
  - os eventos aparecem em português ("Entrou no site", "Regra de descrição criada (22)");
  - o endereço do próprio computador aparece como "Acesso local".

## [2.23.2] — 2026-10-21 · desenvolvimento

### Publicação e documentação
- **A skill `trabalho-longo` é oferecida na hora certa** (#280). Numa sessão com muitas issues pedidas uma de cada vez, ela nunca foi oferecida, e o trabalho parou quando acabou o limite de uso. Agora:
  - o `CLAUDE.md` e a skill têm um critério objetivo: mais de uma issue ou PR, a 2ª issue na mesma conversa, ou conversa já resumida;
  - um hook do Claude Code, versionado em `.claude/settings.json`, lembra automaticamente quando o pedido tem link de issue ou PR ou a sessão fica longa. Vale para todos do projeto; para desligar, use `FINANCAS_SEM_LEMBRETE_TRABALHO_LONGO=1`.

## [2.23.1] — 2026-10-21 · desenvolvimento

### Correções
- **Contas a pagar: selo do banco colado no nome e YouTube com o selo do Google** (#274). O selo do banco e o nome da conta ganham espaço depois do nome da cobrança; e, entre o texto da cobrança, o estabelecimento e o nome, vale a marca mais específica ("GOOGLE" + "Google YouTubePremium" dá YouTube; "PIX ENVIADO" + "Netflix" dá Netflix).

## [2.23.0] — 2026-10-21 · desenvolvimento

### Funcionalidades
- **Investimentos: câmbio do dia** (#247). No Resumo, o bloco "Câmbio" mostra 1 dólar, 1 euro e 100 ienes em reais, pela PTAX de compra do Banco Central que o site já guarda, com a data de cada cotação e o ⓘ da PTAX. Funciona sem internet, com a última cotação e a data dela.

### Correções
- **O ⓘ dos termos fechava sozinho no celular** (#247): a rolagem que vem com o toque fechava a explicação logo depois de abrir. Agora ela acompanha o ⓘ e só fecha se ele sair da tela.

## [2.22.1] — 2026-10-21 · desenvolvimento

### Publicação e documentação
- **CI mais rápida** (#255, ADR 0047): o PR roda só os testes do front-end afetados pelo que mudou (tudo no merge); API, Jest e testes no navegador (em 2 partes) rodam em paralelo; o instalador .msi só é gerado e testado no PR que mexe nele (antes rodava em todo PR, por causa da linha da versão); caches de node_modules, do build do Angular, do Jest e do NuGet no Windows. A imagem Docker do Playwright foi avaliada e descartada (mais lenta de baixar que o navegador em cache).

## [2.22.0] — 2026-10-21 · desenvolvimento

### Funcionalidades
- **Transações: ordenar a lista pelo cabeçalho** (#244). Clicar em Data, Categoria, Conta ou Valor ordena a lista inteira (não só a página), e clicar de novo inverte; a seta mostra a coluna e a direção. Em Valor, o maior lançamento primeiro, gasto ou receita; sem categoria fica no fim. A ordem fica no endereço da página (F5 e link salvo mantêm).

## [2.21.0] — 2026-10-21 · desenvolvimento

### Funcionalidades
- **Mais selos de marcas** (#269): Pix, Mercado Pago ("MP*MP", "MP*NOME"), Uber pelo processador ("DL*UberRides", "UBERTRIP", "Uber One"), Bilhete Único ("BILHUNICO", com um ônibus), Cobasi, Petz, Petlove (com uma pata) e Extra (só nas formas da loja, para "extrato" e "hora extra" não virarem a loja).

## [2.20.0] — 2026-10-20 · desenvolvimento

### Funcionalidades
- **Calendário: escolher o mês e o ano pelo título** (#264). Em todo campo de data, clicar no título do calendário ("outubro de 2026") mostra os 12 meses do ano; clicar de novo ("2026") mostra uma página de 12 anos. Escolher um ano leva aos meses, e um mês aos dias. As setas continuam iguais, e Esc volta um passo antes de fechar. Meses e anos fora do permitido ficam desabilitados.

## [2.19.2] — 2026-10-20 · desenvolvimento

### Correções
- **Calendário do "Até (opcional)" abria em dezembro de 2099** (#263). Em Contas a pagar, com o próximo vencimento no futuro, o calendário do "Até" vazio abria no fim do intervalo permitido. Agora todo campo de data vazio abre no mês de hoje ou, se hoje não pode ser escolhido, no mês da data aceita mais perto de hoje.

## [2.19.1] — 2026-10-20 · desenvolvimento

### Correções
- **Contas e cartões: recorrentes em cima do rodapé e histórico de faturas esticando os outros cartões** (#245). Na conta com cartões ligados e cobranças recorrentes, a linha "N cobranças recorrentes" caía em cima do número da conta e de "transações · editar"; agora cada linha tem o seu lugar. Abrir o "Histórico de faturas" de um cartão aumenta só aquele cartão: os vizinhos da mesma linha ficam como estavam, sem espaço vazio.

## [2.19.0] — 2026-10-20 · desenvolvimento

### Funcionalidades
- **Ícone de cada cobrança em Contas a pagar** (#265): o selo do serviço quando o site o reconhece pelo texto do extrato, no mesmo estilo dos bancos. São 116 marcas comuns no Brasil: streaming e música (Netflix, Spotify, YouTube, Globoplay, Disney+…), telefone e internet (Vivo, TIM, Claro, Oi), transporte e comida (Uber, 99, iFood, Rappi, Sem Parar), compras (Amazon, Mercado Livre, Shopee, Magalu), luz, água e gás (Enel, Sabesp, Cemig…), saúde, academia, viagens, programas e IA; senão, o ícone da categoria (Moradia, Educação, Saúde…). Aparece em Contas a pagar (lista, "Assinaturas e recorrentes" e sugestões do extrato), em Transações (cada linha) e no Painel (próximos vencimentos e últimas transações). Nada é baixado da internet.
- **Contas a pagar com o selo do banco e blocos que abrem e fecham** (#242). O selo do banco ou do cartão aparece ao lado da conta de cada cobrança, nas sugestões do extrato e no título de cada bloco de "Assinaturas e recorrentes". Cada bloco agora abre e fecha pelo título (clique ou teclado). Fechado, ele ainda mostra quantas cobranças tem e o total por mês. Dá para deixar vários abertos, e "Fechar todos" / "Abrir todos" faz de uma vez. O site lembra, neste navegador, quais você fechou.

## [2.18.2] — 2026-10-20 · desenvolvimento

### Correções
- **Projeção e estornos de compra parcelada** (#257). Compra parcelada devolvida inteira (o banco estorna o valor total) agora conta como cancelada: o estorno e as parcelas já lançadas saem dos relatórios. Antes, o estorno do total abatia o gasto de um mês em que só a 1ª parcela tinha sido cobrada, e as despesas do mês ficavam negativas no Painel, nos Orçamentos e na Projeção. Na Projeção, as parcelas futuras do cartão não são mais descontadas duas vezes (no "Disponível hoje" e de novo em cada mês), e o valor negativo não quebra mais a linha entre o "−" e o número.

## [2.18.1] — 2026-10-20 · desenvolvimento

### Correções
- **Sugestões de categoria com regras mais seguras** (#254). A regra sugerida agora sai da descrição do banco, que é o que a regra confere, e não do nome do estabelecimento: antes, um grupo de pagamento de fatura sugeria "contém BANCO", que pegaria Pix e tarifas. A caixa "criar regra" vem desmarcada, com o motivo embaixo, quando a regra já existe, quando o texto também aparece em transações de outras categorias, ou quando outra linha pede a mesma regra para outra categoria. "Aplicar as N de confiança alta" não cria mais duas regras iguais com categorias diferentes. As categorias sugeridas (as bolinhas) não cobrem mais o texto do motivo.

## [2.18.0] — 2026-10-20 · desenvolvimento

### Funcionalidades
- **Avisos do Painel agrupados e que dá para ocultar** (#206): os orçamentos estourados agora ficam num card só, "N orçamentos estourados neste mês", com as piores categorias primeiro. Cada uma mostra quanto passou (em R$ e %), uma barra em que a parte clara é o limite, e o nome leva às despesas dela no mês. As que passaram só por emergência ficam em laranja, e "Ver todos" mostra o resto. Os outros avisos (conta vencida, consentimento, erros) vão para um card próprio. O menu "Ocultar" de cada card tem "Minimizar" (lembrado: o card fica recolhido toda vez que abrir o Painel, até clicar em "Mostrar") e "Ocultar até…" (lembrado). Recolhido, o card vira uma linha com a quantidade. O dos orçamentos volta sozinho no mês seguinte, se outra categoria estourar ou se uma piorar de faixa; o dos outros avisos volta quando aparece um aviso novo.

## [2.17.3] — 2026-10-20 · desenvolvimento

### Correções
- **Painel alinhado** (#246, #258): "posição em … · não muda com o mês escolhido" fica à direita, na linha de "Situação hoje", como o seletor de mês da seção de baixo. Os avisos discretos do topo (descrições pouco informativas, trocos) usam a largura toda e não quebram mais em duas linhas com espaço sobrando.

## [2.17.2] — 2026-10-20 · desenvolvimento

### Publicação e documentação
- **Testes de estilo no navegador** (#204): o visual que os testes antigos só adivinhavam lendo o CSS (cards do mesmo tamanho, colunas, divisórias, avisos centralizados, a seta das listas, as animações e "reduzir movimento") agora é medido na tela de verdade, e cada teste foi conferido estragando a regra de propósito. Os testes antigos saíram (nada ficou sem cobertura). A CI também confirmou que não dá para juntar os testes de instalação do Windows nem reaproveitar o build no instalador; o motivo está na ADR 0029.

## [2.17.1] — 2026-10-20 · desenvolvimento

### Correções
- **Contas e cartões: fatura já paga aparecia como "fechada", e o card não mostrava a fatura seguinte** (#251). O Nubank não manda pelo Open Finance o pagamento da fatura, só o lançamento "Pagamento recebido" no cartão, e esse lançamento não era usado para saber se a fatura foi paga.
  - A fatura passa a contar como paga também pelo pagamento lançado no cartão entre o vencimento anterior e o vencimento dela (pagar antes do fechamento conta). Vale para o card, o Painel ("Faturas do mês"), o histórico de faturas, "Conferir com o banco" e o "fatura a pagar" do plano das metas.
  - Depois que a fatura do mês fecha, o card mostra embaixo a **próxima fatura**: o valor que já entrou nela, quando fecha e quando vence.

## [2.17.0] — 2026-10-20

Primeira versão publicada desde a 2.1.0. O número saltou porque, desde a ADR 0039, cada mudança sobe a versão; as 2.2.0 a 2.16.0 só existiram como versões de desenvolvimento.

### Destaques
- **Os números batem com o app do banco:** fatura do mês pelo ciclo do cartão, estorno abatendo o gasto em vez de virar receita, projeção sem parcelas em dobro, trocos das compras de FII fora dos rendimentos e a tela nova **Conferir com o banco**, que mostra as pistas quando algo não bate.
- **Contas a pagar com assinaturas e débitos automáticos:** como cada conta é cobrada, baixa sozinha quando a cobrança chega, aba **Assinaturas e recorrentes** com o de-para por cartão, avisos de "não veio", "aumentou" e "mudou de cartão", e regras de descrição por valor para separar cobranças de mesmo nome.
- **Investimentos mais completos:** **Mês a mês dos FIIs** (aportes, rendimentos por fundo, valorização e saldo), rendimento por mês da caixinha e dos CDBs, taxa da B3 pelo extrato, Méli Dólar no patrimônio, reservas pelo valor que o banco informa e nomes que se reconhecem ("CDB · Nubank").
- **Metas que cabem no mês:** "quanto guardar" pela sobra real (contando a fatura que vence), plano em degraus e "consigo guardar outro valor".
- **Navegação e visual:** menu lateral em grupos que abrem e fecham, menu que se esconde no computador, campos de valor, data e seleção padronizados, e acessibilidade revisada em todas as telas (contraste, teclado, alvos de toque).
- **Gerenciador também no Linux** e, nas duas janelas, "Atualizar para a última do develop" na instalação de desenvolvimento.
- **Banco:** a API acrescenta sozinha as tabelas e colunas novas ao iniciar (com cópia de segurança antes). Quem instala pelos scripts roda `scripts/geral/2-criar-banco.sh`. Os scripts mudaram de pasta (`scripts/geral/`, `scripts/windows/`, `scripts/linux/`); veja "Scripts separados por sistema" abaixo.

### Publicação e documentação
- **Manual: capítulo novo "Corrigir seus dados"** (#222). Ele vem logo depois de "Primeiros passos" e tem duas partes:
  - **o que já se acertou sozinho** depois das correções: estornos, transferências, pagamento de fatura, investimentos e parcelas;
  - **o passo a passo** para deixar o site igual aos apps dos bancos: sincronizar, conferir as reservas e o Méli Dólar, tirar os trocos dos rendimentos, responder a pergunta do limite, informar o nome nos Pix, cadastrar as assinaturas e terminar em "Conferir com o banco".
- **Skill do projeto `trabalho-longo`** (#217): em `.claude/skills/`, para quem usa o Claude Code no projeto. Ela monta o esquema para um trabalho grande continuar sozinho quando o limite de uso acaba no meio. São três partes:
  - o plano em arquivo, numa pasta ignorada pelo git;
  - o agendamento "continua o plano", da sessão, com as limitações explicadas;
  - o fechamento.
  - Antes só existia na máquina de quem pediu.
- **CI bem mais rápida** (#204): os testes do Jest caíram de ~6 min para menos de meio minuto. Eles carregavam a biblioteca de ícones inteira (10,6 MB, ~1.900 ícones) em cada arquivo de teste; agora usam um substituto leve, e o desenho dos ícones fica com os testes no navegador. A CI também guarda em cache os pacotes NuGet e o navegador dos testes.
- **Scripts separados por sistema** (#154): `scripts/geral/` (bash dos dois sistemas: `instalar.sh`, `1-` a `4-`, `parar.sh`, `comum.sh`), `scripts/windows/` (`instalar-windows.ps1`, `parar-windows.ps1`, `instalar-gerenciador.ps1`) e `scripts/linux/` (`instalar-gerenciador.sh`). O `instalar-windows.cmd` e o `parar-windows.cmd` continuam na raiz e funcionam como antes.
  - **Quem instalou pelos scripts:** os comandos passam a ser, por exemplo, `scripts/geral/3-publicar-atualizar.sh` e `scripts/linux/instalar-gerenciador.sh`. O backup diário já agendado e o comando que o gerenciador do Linux mostrava continuam funcionando por repasses temporários; para gravar o caminho novo no agendamento, rode `scripts/geral/4-backup.sh --agendar` de novo.
- **Testes no navegador** (#185, ADR 0041): a cada PR, a CI abre cada tela do site num navegador de verdade (o build de produção, com a mesma política de segurança da API e dados de exemplo, sem nenhum dado real), no computador e no celular, e falha com erro no console, exceção ou bloqueio da política de segurança. Pega, por exemplo, o erro da #157. Para quem desenvolve: `npm run test:navegador`, prints com `FOTOS=…`, e a skill `testar-no-navegador` do Claude Code.
- **Versões publicadas sempre como "Finanças Local X.Y.Z"** (#187). O workflow de versão acerta o título e as notas mesmo se o Release foi criado à mão antes da tag. As notas começam com **como instalar ou atualizar** e trazem a seção do CHANGELOG sem o cabeçalho repetido.
- **A versão sobe a cada mudança** (#187, ADR 0039): todo PR para o `develop` sobe o número no `Directory.Build.props`, pelo tipo (funcionalidade → o do meio; correção e ajuste → o último), e a CI confere. A versão de desenvolvimento passa a mostrar o número que vai sair, em vez de "2.1.0-dev.N".
- **README mais simples para quem usa** (#187): instalar, conectar os bancos, o que tem, atualizar, desinstalar, segurança e problemas comuns. Instalar pelo código, scripts, Linux, migrar do SQL Server e estrutura de pastas foram para `docs/INSTALACAO-PELO-CODIGO.md`.

### Correções
- **Contas a pagar: a sugestão do extrato voltava depois de cadastrada com outro nome** (#205). "PIX ENVIADO LIVEPIX LTDA" cadastrada como "Manwha" continuava em "Sugestões do extrato", porque só a descrição ligava as duas.
  - **Cadastrar pela sugestão** grava na conta a pagar de qual pagamento do banco ela veio: o nome pode ser qualquer um.
  - **Conta cadastrada à mão** (ou antes desta versão) tira a sugestão quando o valor (até 15%), o dia do vencimento (até 3 dias) e a conta batem.
  - **Na dúvida** (duas contas parecidas, conta a pagar sem a conta escolhida ou com outro texto da cobrança), a sugestão mostra "parece ser *nome*" e o botão **"é a mesma"**, que grava a ligação. O aviso tem "Desfazer".
  - **Baixa sozinha** (débito automático e cartão) também grava a ligação na primeira cobrança casada.
  - **Banco:** coluna nova `ContaAgendada.ChaveExtrato`. A API acrescenta sozinha ao iniciar; quem instala pelos scripts roda `scripts/geral/2-criar-banco.sh`.
- **Investimentos › Reservas: "conferir saldo" voltou nas reservas que o banco não detalha** (#237). Desde a #168, quando a conta tinha o reservado informado pelo Open Finance, todas as reservas dela apareciam como "informado pelo banco" e sem "conferir saldo"; com o banco mandando só o total, as que não eram a de movimento mais recente ficavam com zero. Agora:
  - "informado pelo banco" só na reserva cujo valor veio, pelo nome, da lista detalhada do banco;
  - as outras continuam estimadas e com "conferir saldo". A que não está na lista detalhada avisa que pode estar com outro nome;
  - com só o total, o que ele passa da soma das reservas aparece na linha "Reservado" ("restante do total do banco"), que se ajusta quando você confere as outras.
- **Acessibilidade achada no teste de ponta a ponta** (varredura de todas as telas no navegador, em computador, celular e tema escuro):
  - **Contraste do tema claro** (WCAG 1.4.3): texto suave, verde, rosa, alerta e emergência ficavam abaixo de 4,5:1 sobre os fundos. Estão um pouco mais escuros, em todas as paletas; um teste confere a conta.
  - **Só com o mouse** (WCAG 2.1.1): abrir uma transação, uma categoria ou uma regra de categorização e reutilizar um resultado da calculadora. Agora o nome é um botão (Tab + Enter); o clique na linha continua valendo.
  - **Alvos menores que 24 px** (WCAG 2.5.8): o ✕ dos avisos, "ver todos" do Painel, os cartões das faturas, a navegação do Manual e os botões com cara de link.
  - **Categorias rolava 3 px para o lado** no computador: a dica escondida do ícone na borda direita contava na largura. A dica perto da borda agora abre alinhada pelo lado do ícone.
  - **Metas:** "editar" e "+ aporte" dizem de qual meta são para o leitor de tela.
- **Busca de Transações diferenciava maiúsculas** (#158): "netflix" não achava "NETFLIX.COM". Agora a busca ignora a caixa das letras (sem acento), e % e _ digitados contam como texto.
- **Mais robusto quando o banco manda os dados de outro jeito** (#172):
  - **Pagamento de fatura** que o banco manda como boleto ou "outros" fica fora dos gastos quando o crédito do mesmo valor aparece num cartão seu em até 5 dias. Sem o crédito, fica como está, porque "fatura" também é conta de telefone.
  - **Aplicação e resgate** de RDB, CDB e poupança vão para "Investimentos", por regras padrão novas. Quem já tinha o site as ganha uma vez, e as transações antigas são acertadas, menos as de categoria escolhida à mão.
  - **Pendente antiga** que o banco efetivou com outro código sai sozinha, mesmo antes dos dias relidos, para não contar em dobro.
  - **Seu nome nos Pix:** campo novo em Configurações › Open Finance. Quando o banco não informa o titular, Pix e TED que citam o seu nome contam como entre contas suas (ADR 0031).
  - **Conta em outra moeda** gera um aviso no Painel: o site soma as contas como reais (ADR 0008).
  - **Reserva de emergência** sem ações, FIIs e ETFs, também os do Open Finance, igual à carteira. O ⓘ explica por quê.
  - **Contas fora do saldo e arquivadas** continuam nos relatórios (ADR 0042), e a edição da conta diz isso.
- **Estorno no cartão contava como receita** (#166, ADR 0036). Crédito no cartão (compra devolvida, "crédito de confiança", cashback) virava receita e não descontava o gasto da categoria. Isso inflava a receita, a renda (50/30/20), os orçamentos e o gasto por categoria.
  - **Agora:** o estorno abate o gasto da categoria e não é receita. O pagamento da fatura recebido no cartão fica fora dos gastos mesmo sem a categoria certa.
  - **Estorno do NuPay** que chega como transferência ganha a categoria do texto ("Estorno de Uber" → Transporte).
  - **Relatórios:** passam a somar receita e despesa pelo tipo da transação, e não pelo sinal.
  - **Transações antigas:** as que já existiam são acertadas sozinhas ao iniciar o site.
  - **Manual:** seção nova "Estornos no cartão".
- **Fatura do mês do cartão (ciclo aberto) diferente do app do banco** (#167). Duas causas:
  - **Lançamento da fatura anterior contado de novo:** o banco às vezes manda, datado depois do fechamento, um lançamento que já cobrou na fatura anterior, e ele entrava também na fatura aberta. Agora o que o banco ligou a uma fatura fica nela.
  - **Parcela futura na fatura errada:** o Open Finance pode datar as parcelas futuras um dia depois do fechamento, e cada uma ia para a fatura seguinte. Agora a parcela k vai para a fatura da parcela já cobrada mais a diferença de número.
  - **Onde vale:** Painel (Faturas do mês), Contas e cartões e a sobra das Metas.
- **Projeção contava parcelas futuras em dobro** (#165). Quando duas ou mais parcelas da mesma compra estavam nos últimos 31 dias, cada uma projetava o resto da compra. Isso inclui as que o banco já manda com data futura. Exemplo: um 3× de R$ 100 aparecia com R$ 200 no último mês.
  - **Agora:** de cada compra vale só a parcela mais recente. As que o banco já mandou entram como são, e só o que falta é projetado.
  - **A mesma compra é reconhecida** mesmo com centavos ou descrição diferentes entre as parcelas.
  - **Assistente de metas:** passa a usar a mesma regra.
- **Metas: "quanto guardar este mês" pedia quase tudo de uma vez** (#149, ADR 0035). A sobra do mês não contava a fatura do cartão que vence nele. Num mês de fatura alta, o assistente chegava a pedir ~8× a divisão igual.
  - **Sobra pelo dinheiro que entra e sai:** receitas − fatura que vence no mês − outras despesas. As faturas futuras são estimadas pelas compras à vista e pelas parcelas que ainda faltam, então aparece a queda quando as parcelas acabam.
  - **Limites do mês:** o pedido não passa do que deve sobrar nas contas no fim do mês (já paga a fatura), nem do que sobrou no seu melhor mês recente. Havendo sobra, sugere pelo menos R$ 50. Nenhum mês de uma meta com prazo passa de 2× a divisão igual.
  - **Plano em degraus**, com o motivo: "R$ 510 em nov · R$ 2.290/mês de jan a set (faturas ~R$ 1.500)". Avisa quando o plano conta com a fatura caindo.
  - **"Guardar X% da sobra"** na própria tela de Metas. É a mesma escolha da margem de segurança, e recalcula na hora.
  - **"Consigo guardar outro valor"** em cada meta: o resto do plano é refeito na hora e mostra se ainda fecha no prazo. O valor fica na URL até você registrar o aporte.
  - **Quando não fecha no prazo,** aparecem as saídas: mudar o prazo ou o valor, ver onde cortar (50/30/20) e Orçamentos.
- **Logo do Mercado Pago não aparecia** (#147). O desenho do ícone tinha sido colado cortado em 3000 caracteres, no meio de uma curva. O navegador recusava o desenho e mostrava o erro `<path> attribute d` no console, em Transações, Contas e cartões e no Painel. Agora o ícone está inteiro (Simple Icons). Um teste confere que o desenho de todo banco está completo.
- **"Sincronizado: X atualizada(s)" com o mesmo X toda vez** (#156). A sincronização marcava de novo como transferência os Pix e TEDs para outra pessoa da categoria "Transferências", e a reclassificação do fim os devolvia para despesa ou receita. Eles contavam como "atualizados" em toda sincronização, mesmo sem nada novo no banco. Agora a sincronização só recalcula o tipo de uma transação nova ou que mudou de categoria, e o número conta só o que mudou de verdade.
  - **Junto:** "Ignorar nos relatórios" marcado à mão num Pix para outra pessoa não se perde mais na sincronização seguinte.
- **Erro de CSP no console em toda página** (#157, ADR 0034). O build do Angular escrevia no `index.html` um script com a conferência de integridade (SRI) das páginas, e a política de segurança do site o bloqueava. O SRI foi desligado: aqui todos os arquivos saem do mesmo programa local, e ele não protegia contra nada realista. A política de segurança continua igual, sem permitir script inline.
- **Instalação no Windows falhava com "EPERM ... esbuild.exe"** quando o front-end estava aberto em desenvolvimento (`npm start`). O Windows não apaga um programa em execução, e o `npm ci` precisa apagar a pasta `node_modules`. Agora o `instalar-windows.cmd` e o `3-publicar-atualizar.sh` (no Git Bash) encerram antes só os processos que rodam de dentro de `src/frontend/node_modules` e dizem quais foram.
- **Fatura do mês dos cartões com valor errado** (#144). A fatura do mês somava só as compras de 1 a 31 do mês, então ficava muito abaixo da fatura do banco. Agora ela é a fatura que fecha no mês:
  - **Já fechada:** vale o total que o banco informou.
  - **Ainda aberta:** soma as compras desde o fechamento anterior e a parcela do mês, menos os estornos.
  - **Na tela:** o Painel e Contas e cartões mostram quando a fatura fecha e quando vence.
- **Ícone de "ocultar valores" parecia invertido.** Com os valores ocultos, o botão ficava destacado com o olho aberto. Agora o ícone mostra o estado, como nos apps de banco: olho riscado (e botão destacado) quando os valores estão ocultos, e olho aberto quando estão à mostra. A dica continua dizendo a ação do clique.
- **Radio buttons:** ficavam esticados na largura toda (Renomear transação). Agora têm o visual da caixa de marcar nova, redondos.
- **Listas suspensas no visual do site:** o campo fechado ganhou uma seta própria, no lugar da caixa do sistema, e fica numa linha só. A lista que abre continua a do navegador, e o teclado, o leitor de tela e o celular funcionam como antes.
- **O que aparece depois entra com um fade curto,** em vez de surgir de uma vez: botões, avisos, etiquetas, cards, blocos, menus que abrem, listas vazias e mensagens de erro. Nada anima com "reduzir movimento" no sistema.
- **Caixa de marcar nova e seleção de várias linhas melhor** (#133, ADR 0032).
  - **Caixa nova em todo o site:** no visual do site, nos temas claro e escuro, com os estados marcada, parcial (traço) e desabilitada.
  - **Nas tabelas:** a célula inteira é clicável e a linha selecionada fica destacada (Transações, Investimentos › importar do extrato, Sugestão de orçamentos, Conferir investimentos).
  - **Em Transações:**
    - Shift+clique seleciona um intervalo; Esc limpa;
    - "Selecionar todas desta página" mostra o estado parcial;
    - a barra diz "5 selecionadas (2 em outras páginas)", abre com animação, sem empurrar a tabela de uma vez, e fica fixa no topo ao rolar.
  - **Trocar de página mantém a seleção;** trocar o filtro limpa, com o aviso "Seleção limpa ao trocar o filtro".
  - **Leitor de tela:** cada caixa diz qual é a transação ("Selecionar: PIX ENVIADO JOAO, 12/10/2026, R$ 150,00"), e a contagem é anunciada.
- **Campo de valor em reais padrão** (#132, ADR 0032), em todos os campos de dinheiro: Transações, Contas e cartões, Contas a pagar, Metas, Orçamentos, Investimentos e Conferir investimentos.
  - **Entende o jeito brasileiro:** "1.234,56", "1234,56", "R$ 1.234,56" (colado do app do banco) e "1234.56". "1.500" é mil e quinhentos, não 1,5.
  - **Formata enquanto você digita:** "124214" já aparece como "124.214", com o cursor no lugar. Ao sair do campo, completa as casas ("124.214,00").
  - O símbolo da moeda fica dentro do campo. Investimentos aceitam outra moeda e até 4 casas.
  - **No celular**, abre o teclado numérico. **No computador**, a rodinha do mouse não muda mais o valor, e as setinhas saíram.
  - **Erro em texto, perto do campo:** "Informe um valor maior que zero.".
  - **Orçamentos:** continua salvando ao sair da linha, e agora também com Enter.
  - Os valores enviados à API não mudam.
- **Interruptor para o que muda a tela na hora** (#130, ADR 0032).
  - "Mostrar arquivadas" (Contas e cartões), "Mostrar encerradas" (Investimentos › Posições) e "Agrupar subcategorias" (Relatórios) viraram interruptores. As caixas de marcar de formulário, com "Salvar", não mudam.
  - **"Mostrar arquivadas":**
    - o estado fica na URL (`?arquivadas=1`), e recarregar, voltar ou abrir um link salvo mostra a tela do mesmo jeito;
    - conta ou cartão arquivado mostra o selo **"Arquivada"** em texto.
  - Os filtros de Transações continuam caixas de marcar, por consistência com a barra de filtros (o porquê está na ADR 0032).
- **Esconder o menu lateral no computador** (#131).
  - **Botão ☰:** o mesmo botão da barra de cima, que antes só aparecia no celular, agora também esconde e mostra o menu no computador. O conteúdo passa a usar a largura toda, e os gráficos se redesenham sozinhos.
  - **Nome acessível:** fixo, "Menu de navegação", e o leitor de tela ouve pelo `aria-expanded` se o menu está aberto ou fechado.
  - **Menu escondido:** sai da ordem do Tab (`inert`), tanto no computador quanto com a gaveta do celular fechada.
  - **A escolha fica salva neste navegador** (`financas.menuRecolhido`, ADR 0017), e o padrão é o menu aberto. No celular a gaveta continua começando fechada.
  - **Com o menu escondido, a página ganha a largura do menu** (até 1632 px) e fica **centralizada**, sem encostar à esquerda com um vazio grande à direita em tela larga.
  - **Animação:**
    - ao esconder, o menu desliza enquanto a coluna encolhe;
    - ao mostrar, a coluna abre de uma vez e o conteúdo, o ☰ e o título deslizam junto com o menu, só com `transform` (sem o pisca-pisca de refazer o layout a cada quadro);
    - nada disso acontece com movimento reduzido no sistema.
  - **Gaveta do celular:** dá para fechar sem escolher uma tela. As formas são:
    - o **✕ "Fechar menu"**;
    - **tocar no fundo escuro**;
    - **arrastar para a esquerda**;
    - **Esc**, que também devolve o foco ao ☰.

    A gaveta ocupa no máximo 85% da tela. Aberta, o foco vai para o ✕ e o conteúdo atrás sai do Tab.
- **A marca "Finanças" leva ao Painel:** no topo do menu e no título da barra de cima (que aparece no celular e com o menu escondido). No celular, clicar nela também fecha a gaveta.
  - O título da barra tem o **cofrinho** da marca e entra e sai com fade.
- **Transição entre telas:** a tela nova entra com um fade curto e sobe 8 px.
  - Só ao trocar de tela; filtros, página da tabela e âncoras do manual não animam.
  - Sem animação com movimento reduzido no sistema.
- **Gerenciador também na instalação pelos scripts** (Windows, ADR 0028).
  - **Como instalar:** `scripts\instalar-gerenciador.ps1` publica o gerenciador em `publicado\gerenciador` e cria o atalho no Menu Iniciar, sem administrador.
  - **Como acha o site:** pelo serviço `FinancasLocal`, e o banco pelo `appsettings` da instalação.
  - **Versão nova:** nessa instalação, ele não oferece o `.msi` e diz como atualizar pelos scripts.
- **Visual novo do gerenciador e do instalador**, com a identidade do site.
  - **Gerenciador:** cabeçalho com a marca, cartões, botões arredondados e um ponto colorido com a situação. Segue o tema claro ou escuro do Windows, inclusive na barra de título. As preferências viraram interruptores (liga/desliga), e o botão em destaque é a próxima ação provável: "Abrir o site" quando está no ar, "Ligar" quando está desligado.
  - **Instalador `.msi`:** imagens com o degradê da marca e o cofrinho nas telas de boas-vindas, de conclusão e no topo das demais (`instalador/imagens`).
- **Instalador `.msi` do `develop`, com link fixo** (ADR 0029). A cada merge no `develop`, depois dos testes de instalação, a CI publica o `FinancasLocal-desenvolvimento.msi` no pré-lançamento [desenvolvimento](https://github.com/LucasDesenvGit/financas-local/releases/tag/desenvolvimento).
  - **Versão mostrada:** `X.Y.Z-dev.N`, no site e no gerenciador.
  - **Troca entre as duas:** o `.msi` de mesma versão `X.Y.Z` substitui o instalado, e a versão publicada e a de desenvolvimento trocam uma pela outra sem ficarem as duas instaladas. A CI testa essa troca.
  - **Avisos do gerenciador:** continuam só para as versões publicadas.
- **Mais fácil sair da instalação pelos scripts:** o gerenciador dessa instalação diz que o `.msi` traz o banco e as chaves e tem o botão **Baixar o instalador .msi**.
- **Atalho do gerenciador na área de trabalho**, para abrir com um clique.
  - **`.msi`:** o atalho vai para a área de trabalho de todos os usuários. Com `ATALHOAREADETRABALHO=0`, instala sem ele. A desinstalação remove o atalho.
  - **`instalar-gerenciador.ps1`:** o atalho vai para a área de trabalho do usuário. Com `-SemAtalhoNaAreaDeTrabalho`, não cria.

### Funcionalidades
- **Regras de descrição por valor** (#241, ADR 0030 e 0043): separam cobranças que chegam do banco com a mesma descrição e só mudam no valor, como duas contas do condomínio.
  - **Em Categorias e regras › Regras de descrição**, o grupo novo **"E o valor (opcional)"**: "De" e "Até", um dos dois basta. A regra só vale com o valor, sem o sinal, na faixa, e vem antes das regras sem valor. A lista ganhou a coluna "Valor", e a prévia já conta só as transações da faixa.
  - **Em Contas a pagar**, o nome dado por uma regra com valor é o que liga as cobranças: as "Sugestões do extrato" mostram as duas separadas, com o texto da cobrança igual ao nome, e a baixa sozinha e os avisos não misturam mais uma com a outra.
  - **Já tinha cadastrado as contas?** Troque o texto da cobrança de cada uma pelo nome da regra (ex.: CONDOMINIO — TAXA). Um texto que as duas têm (só CONDOMINIO) continua ligando as duas.
  - **Banco:** colunas novas `RegraDescricao.ValorMinimo` e `ValorMaximo`. A API acrescenta sozinha ao iniciar; quem instala pelos scripts roda `scripts/geral/2-criar-banco.sh`.
- **Menu lateral em grupos que abrem e fecham** (#227, ADR 0046). As telas ficam juntas por assunto, com os mesmos nomes do roteiro de primeiro uso: o Painel no topo; **Movimentação** (Transações, Contas e cartões, Contas a pagar); **Planejamento** (Orçamentos, Metas); **Patrimônio** (Investimentos, Empréstimos e financiamentos); **Análises** (Relatórios, Visão anual, Projeção); **Ajustes** (Categorias e regras, Configurações); e o Manual no rodapé.
  - Clique (ou Enter e Espaço) no nome do grupo mostra ou esconde as telas dele. Vários grupos podem ficar abertos; na primeira vez, todos vêm abertos.
  - A escolha fica salva neste navegador. Ao entrar numa tela, o grupo dela abre sozinho; com o grupo fechado, o nome dele fica destacado, para mostrar onde você está.
  - No celular, tocar no grupo não fecha a gaveta; tocar numa tela, sim. O ☰ do computador continua escondendo o menu inteiro.
- **Rendimento por mês da caixinha e dos CDBs** (#173 e, ADR 0045; tabela nova `HistoricoInvestimento`). O Open Finance manda só o saldo de hoje; agora a sincronização guarda um registro por investimento e dia. Em Investimentos › Open Finance, "Rendimento por mês" mostra quanto os investimentos renderam em cada mês (quanto o saldo cresceu além do valor aplicado) e o saldo no fim, com "por investimento". O histórico começa na atualização: o primeiro mês conta a partir daquele dia.
- **"por fundo" dos FIIs** lido certo pelo leitor de tela: o nome do fundo e o valor saíam colados.
- **Trocos classificados sozinhos na sincronização** (#173 b, ADR 0044; em parte substitui a ADR 0027). Depois de cada sincronização, o troco "sem dúvida" das compras de FIIs e ações vai para "Investimentos" e deixa de contar como rendimento, como no app. É "sem dúvida" quando o dia de compras tem pelo menos 5 dias, todas as compras têm operação e a conta do dia sobra só a taxa da B3. Crédito com categoria escolhida à mão e valor de venda ficam de fora. O Painel avisa, com "Ver em Transações" (só esses lançamentos), "Desfazer" (volta e não muda de novo) e "Ok".
- **Taxa da B3 pelo extrato** (#173 a, ADR 0027): "Conferir investimentos" ganhou o grupo "Taxa da B3 que falta nas compras". Em cada dia de compras, o que saiu da conta menos o troco e menos cotas × preço é a taxa da bolsa, mostrada com a conta inteira. Ela entra no custo e no preço médio da compra (o "valor final da compra" do app), só se você confirmar, e tem "Desfazer"; o mês a mês continua com cotas × preço, como as "Entradas" do app. Só aparece com o dia já sem troco pendente (5 dias) e até 1% do valor ou R$ 5.
- **Do mês a mês dos FIIs para os lançamentos** (#173 d, ADR 0018): "ver lançamentos", em Aportes e em Rendimentos de cada mês, abre Transações no mês e só com os lançamentos que formam o número. O filtro aparece num chip ("Rendimentos de FIIs: 2 lançamentos") com ✕, e o voltar do navegador leva de volta aos FIIs. Clicar num mês do gráfico abre os rendimentos dele.
- **Rendimentos por FII no mês a mês** (#173 c): "por fundo", embaixo dos rendimentos de cada mês, abre quanto cada FII pagou. O crédito sem o código do fundo é ligado pelo valor (cotas na data-com × valor por cota, pago até 40 dias depois). O que não liga aparece como "sem fundo identificado", sem sumir da soma.
- **Avisos das recorrentes** (#158, parte 3), no topo de Contas a pagar:
  - **"não veio":** a cobrança automática passou 5 dias do vencimento.
  - **"mudou de cartão":** com "Atualizar o catálogo".
  - **"aumentou" ou "baixou":** mais de 5%, menos para quem varia todo mês; com "Atualizar o valor".
  - **"cartão arquivado":** com cobranças ainda nele.
  - **No Painel:** só "não veio" e "cartão arquivado".
  - **Ao arquivar** um cartão ou conta, a edição lista o que ainda cobra ali.
- **Investimentos › FIIs › Mês a mês** (#173), como no app da corretora: por mês, os **aportes** (compras), os **rendimentos** que caíram na conta, a **valorização** das cotas e o **saldo no fim**, num gráfico com a frase-resumo do último mês e numa tabela com o mês mais recente primeiro.
  - **Rendimentos:** os créditos de provento do extrato, sem o troco das compras nem o valor das vendas (a mesma separação da "Conferir investimentos"), valendo mesmo sem a conferência aplicada. No mês atual, o provento com data-com no mês aparece como "a receber".
  - **Subscrição e vendas** aparecem à parte, embaixo dos aportes.
  - **Valorização:** pelo fechamento de cada mês (no mês atual, a cotação de hoje); pode diferir um pouco do app, que usa outra fonte de preço. O termo tem ⓘ com a definição.
  - **Ainda não:** o rendimento das caixinhas (precisa guardar o histórico do saldo) e a taxa da B3 calculada pelo extrato ficam para os próximos passos da #173.
- **Assinaturas e recorrentes** (#158, parte 2): aba nova em Contas a pagar com o **de-para** de cada cobrança que se repete.
  - **Topo:** o total por mês e por ano.
  - **Um bloco por cartão ou conta:** cada cobrança, como é cobrada, o dia, a última cobrança e o valor.
  - **Busca e filtros:** "Onde cai…?", conta ou cartão e forma de cobrança, com chips, na URL. "Ver as cobranças" abre Transações filtrada.
  - **Em Contas e cartões:** a linha "N assinaturas · R$ X/mês" em cada cartão e conta.
  - **Manual:** seção nova "Assinaturas e recorrentes".
- **Assinaturas e débitos automáticos em Contas a pagar** (#158, parte 1; ADR 0043):
  - **Como é cobrada:** cada conta a pagar diz se é paga à mão (boleto, Pix), por débito automático ou no cartão de crédito, e onde cai. Também ganha uma observação.
  - **Baixa sozinha:** com o "texto da cobrança" (a descrição do banco contém…), a conta cobrada sozinha se dá como paga quando a cobrança chega no extrato, sem "Pagar". Vale pela descrição original, mesmo depois de renomear.
  - **Sugestões do extrato** agora acham as **assinaturas nos cartões** (valor igual, até 5% de diferença; sem parcelas nem gastos frequentes), as **anuais** e os **débitos automáticos**, já com a forma e o texto preenchidos.
  - **Painel:** o que cai no cartão sai dos "Próximos vencimentos", porque entra na fatura.
  - **Manual:** seção nova "Assinaturas e débitos automáticos".
- **Conferir com o banco** (#172), tela nova aberta por Contas e cartões.
  - **O que faz:** você digita o que o app de cada banco mostra (saldo de cada conta, fatura do mês de cada cartão, valor de cada investimento e reserva), e o site compara na hora.
  - **Quando não bate,** mostra as pistas: quando o banco atualizou, reservas, pendentes, parcelas, estornos e compras repetidas. A pista com o mesmo valor da diferença vem primeiro, com o link para o detalhe.
  - **Nada é gravado.**
  - **Manual:** seção nova "Conferir com o banco".
- **Gerenciador: "Atualizar para a última do develop", só na instalação de desenvolvimento** (#154, ADR 0040). Quem instalou uma versão publicada não vê nada novo.
  - **`.msi` de desenvolvimento** (versão `X.Y.Z-dev.N`): o gerenciador procura o pré-lançamento "desenvolvimento", avisa quando tem um mais novo e atualiza como na versão publicada, conferindo o SHA256.
  - **Pelos scripts, num clone na branch `develop`:** o botão traz o `develop` do GitHub e reinstala, mostrando a saída ao vivo. Com alteração local, outra branch ou commits que não estão no GitHub, recusa e diz o motivo, sem mexer em nada. No Windows, o git roda como você e só a reinstalação pede permissão de administrador. No fim, "Atualizar o gerenciador e reabrir".
  - **No terminal:** `scripts/geral/atualizar.sh` e `scripts/windows/atualizar-windows.ps1` fazem o mesmo.
- **Limite do cartão reduzido no app** (#171). Quem reduz o limite no app do banco, por segurança, via o "limite usado" muito maior que a dívida do cartão, e o site não sabia por quê.
  - **Agora o cartão pergunta**, no card e na edição: "Você reduziu o limite no app?".
  - **"Sim":** vale o limite que ficou no app.
  - **"Não":** a diferença conta como compromisso, com o limite aprovado. A pergunta só volta se a diferença mudar.
  - **A diferença aparece em texto**, com o termo "limite usado" explicado.
  - **Manual:** seção nova "Limite reduzido no app do banco".
- **Méli Dólar no patrimônio** (#169, ADR 0038). O dólar digital do Mercado Pago não aparecia em lugar nenhum, porque o Open Finance não o informa.
  - **Em Investimentos › Reservas:** a linha "Méli Dólar", com a quantidade em dólar estimada pelas compras e vendas do extrato (pelo "MUSD" da descrição ou pela PTAX do dia), e o saldo em reais pela última PTAX. Entra no patrimônio e no total investido.
  - **Conferir saldo:** em reais ou em dólares; em reais, convertido pela PTAX da data.
  - **Compra e venda de Méli Dólar vão para "Investimentos"** e deixam de contar como despesa ou receita. É uma regra padrão nova, criada uma vez para quem já tinha o site, e as transações já importadas são acertadas, menos as de categoria escolhida à mão.
  - **Manual:** seção nova "Méli Dólar (dólar do Mercado Pago)" e o termo "PTAX" no glossário.
- **Investimentos com nome que se reconhece** (#170). A caixinha do Nubank aparecia como "CDB - NU FINANCEIRA S.A. - SOCIEDADE…", e o CDB do Mercado Pago como "CDB - MERCADO CREDITO…".
  - **Agora:** "CDB · Nubank" e "CDB · Mercado Pago", com o nome do banco embaixo, para achar no app.
  - **Renomear:** cada investimento do Open Finance pode ganhar o seu nome ("Caixinha viagem"), e a sincronização não muda esse nome.
  - **Conexões com o nome do banco:** pelo Meu Pluggy todas se chamavam "MeuPluggy". Agora mostram o banco, tirado das contas da conexão, em Configurações, no histórico de sincronizações e nos avisos do Painel.
  - **Aviso de troco no Painel:** os trocos das compras de FII contando como rendimento geram um aviso, com o link para "Conferir investimentos". A correção continua com prévia (ADR 0027).
  - **Manual:** seção nova "Caixinhas, CDBs e outros investimentos do banco".
- **Reservas e caixinhas pelo valor que o banco informa** (#168, ADR 0037). Quando a instituição manda o valor reservado pelo Open Finance (é o "Reservado" que o MeuPluggy mostra), ele vale no lugar da estimativa pelo extrato. A reserva aparece como "informado pelo banco" e entra certa no patrimônio, sem conferência.
  - Os bancos que não informam continuam com a estimativa e a conferência.
  - **Conferência com valor fora do esperado** (ex.: "10.123,42" gravado como 10,12) pede confirmação antes de salvar.
- **Visão anual › 50/30/20 clicável** (#150): clicar em Essenciais, Estilo de vida ou Guardado (na barra ou na legenda abaixo do gráfico) abre Transações com o que forma aquela parte, de janeiro até o mês do gráfico.
  - **Essenciais e Estilo de vida:** as despesas pela natureza do gasto da categoria (ou da categoria pai, se a subcategoria não tiver). Estilo de vida leva junto o que está sem classificação, como a barra; cada um aparece como filtro removível em "Filtrando:".
  - **Guardado:** as receitas e as despesas do período; o "Resultado" de Transações é o que sobrou.
  - **Correção junto:** uma subcategoria com natureza diferente da categoria pai passava a contar na natureza errada no gráfico. Agora conta na dela, e o total do gráfico bate com a lista de Transações.
- **Relatórios › Maiores despesas clicáveis** (#141): clicar numa despesa abre Transações no dia e na conta dela, com a linha da despesa destacada e o foco nela. Se a despesa não estiver mais lá (apagada, ou mudou de data ou de conta), aparece um aviso. Trocar o filtro tira o destaque.
- **Gerenciador no Linux** (#138, ADR 0033), na instalação pelos scripts.
  - **Como instalar:** `scripts/instalar-gerenciador.sh`, sem `sudo`. Publica em `publicado/gerenciador` e cria o atalho no menu de aplicativos e na área de trabalho (`--sem-atalho-na-area-de-trabalho` para não criar este último).
  - **Na janela:** a mesma do Windows, nas cores do site e no tema claro ou escuro do sistema. Mostra a situação e a versão; liga, desliga e reinicia o serviço do systemd do usuário, sem senha; copia o banco (o arquivo fica legível só por você).
  - **Por que não subiu:** "Ver o registro do serviço" mostra as últimas linhas do journald.
  - **Versão nova:** o comando para atualizar pelos scripts, com **Copiar o comando**. O gerenciador do Linux não acessa a internet.
  - **Site no ar sem login:** avisa quando o serviço para ao sair da conta e oferece **Manter o site no ar sem login** (`loginctl enable-linger`).
  - **Iniciar com o sistema:** ícone na bandeja e aviso (`notify-send`) se o site cair.
  - **No terminal:** `--situacao` mostra a situação sem abrir a janela. A CI do Linux usa esse modo.
  - **Por dentro:** a lógica comum aos dois gerenciadores foi para o `Financas.Gerenciador.Nucleo`. O do Windows funciona como antes.
- **Campo de data novo, com calendário do site** (ADR 0032), em todas as telas.
  - **Digitar:** dd/mm/aaaa, com as barras entrando sozinhas. O ano tem sempre 4 dígitos (antes o campo aceitava "275760"), e colar uma data AAAA-MM-DD também funciona.
  - **Calendário:** botão ao lado do campo, no visual do site, com ‹ › para o mês, « » para o ano, "Hoje" e "Limpar". Pelo teclado: setas, PageUp/PageDown e Esc.
  - **Datas fora do intervalo** (1900–2099, ou o limite da tela, como "até hoje") mostram a mensagem em texto e não deixam salvar.
- **Lista suspensa aberta no visual do site:** no computador, a lista que abre segue o visual do site, com a opção escolhida destacada e as teclas de sempre (↑/↓, Enter, Esc, letra). No celular continua a do sistema.
- **Seletor de mês** (Painel, Orçamentos): clicar no nome do mês abre um quadro com o ano e os 12 meses, para pular vários meses de uma vez. Tem também "Este mês".
- **Regiões que abrem e fecham no meio da página** abrem e recolhem com animação, sem a tela abaixo pular: a barra do modo edição, os filtros ativos, as dicas, os avisos de formulário e o histórico de faturas.
- **Metas:** a sobra prevista fica à direita do título, em verde quando sobra e em vermelho quando falta.
- **Configurações › Conexões:**
  - a situação aparece em português ("Atualizada", "Erro de login no banco", "Precisa de ação sua"…), não mais o código da Pluggy;
  - as ações viraram botões ("Sincronizar", "Pausar" e "Remover", este no estilo de perigo);
  - o campo de adicionar tem rótulo e dica;
  - o "Histórico de sincronizações" ganhou o visual do "Mais análises".
- **O calendário abre por cima de tudo:** dentro de um modal, ele não é mais cortado.

### Correções (layout)
- **Painel: cards do mês reorganizados.**
  - **Antes:** Receitas, Despesas e Resultado esticavam até a altura de "Faturas do mês" (que lista os cartões), com o valor solto no meio de um card alto e vazio.
  - **Agora:** os três ficam em colunas iguais e com a mesma altura, com o valor logo abaixo do título. "Faturas do mês" vai para uma linha própria, na largura toda, e os cartões dividem essa largura lado a lado (2 cartões = metade cada).
  - **No celular**, tudo fica numa coluna.
- **Divisórias em todas as telas.** Linhas finas nos dois temas:
  - abaixo do cabeçalho de cada página;
  - entre as seções de uma tela ("Situação hoje" e "Mês de …" no Painel; "Contas" e "Cartões de crédito" em Contas e cartões);
  - abaixo do título de cada bloco: os de `.bloco-titulo`, os de Investimentos e FIIs e os que começam com o título solto (Visão anual, Relatórios, Projeção, Configurações, Aparência, Parametrização, Manual…);
  - no Painel, em "Faturas do mês", abaixo do total e entre os cartões.
- **Contas e cartões: cards do mesmo tamanho e alinhados.**
  - **Cartões de crédito:** os da mesma linha têm a mesma altura, com "transações · editar" sempre no rodapé. Abrir o histórico de faturas de um cartão também estica os vizinhos da linha.
  - **Contas:** cabeçalho, saldo, "saldo atual" e rodapé ficam na mesma altura em todos os cards da linha, mesmo quando um nome de banco longo quebra em várias linhas.
- **Campos de data com ano de 4 dígitos.**
  - **Antes:** o calendário do navegador aceitava ano com até 6 dígitos ("13/02/275760").
  - **Agora:** todo campo de data vai de 01/01/1900 a 31/12/2099, ou usa o limite da própria tela (ex.: não aceitar data no futuro em Investimentos). Assim o navegador limita o ano a 4 dígitos.
  - Data fora do intervalo não deixa salvar.
- **Lista vazia numa grade de cartões** (ex.: Metas sem nenhuma meta): o aviso ocupa a linha toda, numa linha só e centralizado. Antes ficava preso na primeira coluna, quebrado em duas linhas e fora do meio.

## [2.1.0] — 2026-10-18

Primeira versão com o instalador `.msi` para Windows e o Gerenciador do Finanças Local.

### Funcionalidades
- Transações: **renomear com um nome claro, também para as próximas** (#123, ADR 0030).
  - **Botão ✎ "Renomear"** em cada transação. A janela mostra a descrição do banco e oferece "Só nesta transação" ou "Também nas outras com esta descrição do banco e nas próximas que chegarem".
  - **Regra:** a descrição do banco "for igual a", "contiver" ou "começar com" um texto. Para "contiver", o site sugere o texto sem os números que mudam a cada mês.
  - **Antes de salvar:** prévia com quantas transações vão mudar. Depois, aviso com **Desfazer** e "Voltar à descrição do banco".
  - **Na lista**, o nome novo fica em cima e a **descrição do banco embaixo**, menor ("do banco: …").
  - **Busca** pelos dois nomes. A categoria continua vindo da descrição do banco.
  - **Em massa:** "✎ Renomear" na barra de seleção dá o mesmo nome às selecionadas. Pode ser só nelas, ou com uma regra "é igual a" por descrição do banco, que vale nas outras iguais e nas próximas. O aviso tem "Desfazer".
  - **Onde ficam as regras:** Categorias › Regras automáticas › "Regras de descrição". Lá dá para criar a regra direto com "+ Regra de descrição" (com prévia de quantas transações renomeia), editar, desligar e excluir. Nome escolhido à mão nunca é trocado por regra.
  - **Banco:** tabela nova `RegraDescricao` e colunas `Transacao.RegraDescricaoId` e `Transacao.DescricaoManual` (rodar `scripts/2-criar-banco.sh`).
- Transações: o **valor do que está "fora dos gastos"** (pagamento de fatura, transferência) aparece em cor neutra, para não parecer receita ou despesa.
- **Instalador `.msi` para Windows** (#119, ADR 0029).
  - **Instalação:** dois cliques e "Avançar", sem .NET, Node, Git nem terminal. O site e o gerenciador já vêm compilados para `C:\Program Files\FinancasLocal`.
  - **Dados:** ficam em `C:\ProgramData\FinancasLocal\dados`, só para o sistema e os administradores.
  - **O que o instalador configura:** serviço `FinancasLocal` com reinício automático, certificado HTTPS confiável para `127.0.0.1` e `localhost`, atalhos no Menu Iniciar e, no fim, uma caixa para abrir o gerenciador.
  - **Atualizar:** pelo gerenciador ou abrindo o `.msi` novo; os dados ficam.
  - **Desinstalar:** mantém os dados (`APAGARDADOS=1` apaga também).
  - **Migração:** quem instalou pelos scripts é migrado sozinho. Banco e chaves são copiados, a pasta antiga fica.
  - **Workflow "Versão (instalador .msi)":** gera e testa o pacote num Windows limpo (instalar, atualizar mantendo o usuário, desinstalar mantendo os dados e migrar da instalação pelos scripts). Na tag `vX.Y.Z`, publica o `.msi` e o `SHA256SUMS.txt` no Release. Passo a passo em `CONTRIBUINDO.md`.
  - **Nova configuração na API:** `Dados:Pasta` define banco e chaves numa pasta só e lê o `configuracao.json` dela (certificado). A instalação pelos scripts não muda.
- **Gerenciador do Finanças Local** para Windows (#119, ADR 0028). Instalado pelo `.msi`, roda como o usuário e pede administrador só quando precisa.
  - **Situação do site em palavras:** "no ar" (com a versão do site), "desligado" ou "ligado, mas não responde".
  - **Ações:** "Abrir o site", "Ligar/Desligar" e "Reiniciar".
  - **Versão nova:** procura no GitHub, mostra o que mudou e "Atualizar para x.y.z". Baixa, confere o SHA256 e abre o instalador.
  - **Em "Mais opções":** salvar uma cópia do banco onde você escolher, ver os erros do site, abrir o manual, iniciar com o Windows na área de notificação (com aviso quando o site cai) e procurar versão nova ao abrir.
- **Versão do site num lugar só:** `Directory.Build.props` (`2.1.0`) vale para a API e o gerenciador. Ela aparece em Configurações › Parametrização › Sistema, em `GET /api/auth/estado` (campo `versao`, sem login, só para o gerenciador mostrar junto de "no ar") e no gerenciador.
- **A API cria e atualiza o banco sozinha ao iniciar** (#119, ADR 0003 atualizada).
  - **Os mesmos scripts:** `01-estrutura.sql`, as colunas novas do `2-criar-banco.sh` e, só no banco novo, os dados iniciais, embutidos no build.
  - **Antes de mudar um banco existente:** cópia em `dados/backups/FinancasLocal-antes-atualizacao-*.db`, com as 5 mais recentes guardadas.
  - **Reinício sem mudança de scripts:** não faz nada (`PRAGMA user_version`).
  - **Para que serve:** é a base do instalador `.msi`, que não tem `sqlite3` nem bash. Também deixa o banco pronto no primeiro "F5" do Visual Studio.
- Investimentos: **"Conferir investimentos"** (ADR 0027).
  - **O que faz:** compara a carteira com a posição da corretora e com o extrato, e mostra as correções já marcadas. Nada muda até "Corrigir selecionados", e o aviso tem **Desfazer** por 10 minutos.
  - **FII cadastrado como outro tipo:** reconhecido pelo nome do fundo ou pelo emissor da corretora; passa a ser FII.
  - **Cotas diferentes da corretora:** quando faltam cotas, sugere o débito "Compra de Renda Variável" que nenhuma operação explica. O preço é o débito menos o troco, dividido pelas cotas, comparado com o fechamento do dia. A compra fica ligada ao débito. Sem débito, "Lançar a compra à mão"; com cotas a mais, "Ver as operações".
  - **Troco ou venda contando como rendimento:** o Nubank usa "Valor recebido de Investimentos" para o troco das compras, para o valor das vendas e para os proventos. Troco e venda vão para "Investimentos", com ⓘ "Troco da compra" no glossário.
  - **Rendimento contando como investimento:** os outros créditos voltam para a categoria das regras (Rendimentos).
  - **Aviso no topo de Investimentos** quando a corretora informa outra quantidade de cotas.
  - **Manual:** seção nova em Investimentos.
  - API: `GET /api/carteira/conferencia`, `POST /api/carteira/conferencia/corrigir` e `POST /api/carteira/conferencia/desfazer/{chave}`.
- **Manual do site** (#113, ADR 0026).
  - **Página "Manual"** no menu, com índice de 14 capítulos e busca (sem acento nem maiúscula fazendo diferença, `?busca=` na URL).
  - **Cada seção:** para que serve, passo a passo, "Abrir a tela" (já filtrada quando faz sentido), dicas, erros comuns e termos do glossário.
  - **Endereço próprio por seção:** `/manual/<capitulo>#<secao>`, com navegação entre capítulos.
  - **Botão "Ajuda"** na barra de cima: abre a parte do manual que explica a tela aberta.
  - **Testes que mantêm o manual em dia:** toda tela do menu tem capítulo, e todo link aponta para uma tela que existe.
- Transações: **tipo de cada lançamento** (#111, ADR 0025).
  - **Em cada linha:** etiqueta com ícone e texto: compra no cartão, estorno, pagamento de fatura, Pix enviado ou recebido, transferência, **entre suas contas**, boleto ou débito automático, compra no débito, saque, depósito, tarifa ou juros, rendimento, aplicação ou resgate, salário, empréstimo ou crédito, dinheiro, outro.
  - **De onde vem:** do Open Finance (`operationType`/`paymentMethod` da Pluggy; Pix com o mesmo titular nas duas pontas vira "entre suas contas"); quando o banco não informa, é reconhecido pela descrição.
  - **Fora dos totais:** "fora dos gastos" quando não entra nos totais. **"Somando nos gastos"** ou **"somando nas receitas"** quando um pagamento de fatura ou uma transferência entre contas suas está contando em dobro.
  - **Filtro "Lançamento"** com chip e na URL (`?lancamento=pix`), que vale também no "todas do filtro" do modo edição.
  - **Coluna Conta:** selo do banco e "cartão de crédito" quando é cartão.
  - **Dados guardados:** só o meio e o "mesmo titular" (nunca nomes ou CPF de terceiros), nas colunas novas `Transacao.MeioPagamento` e `Transacao.MesmaTitularidade` (rodar `scripts/2-criar-banco.sh`). As transações antigas ganham o dado do banco quando a sincronização as relê.
- **Contas e cartões** reorganizada (ADR 0024).
  - **Banco de cada conta:** selo com a cor e a marca do banco (Nubank, Itaú, Mercado Pago e mais 25), reconhecido pelo nome da conta e da instituição, ou escolhido em "editar" ("Outro" desliga). O cartão pode ser ligado à conta dele e herda o banco dela, e o cartão desenhado fica com a cor do banco. O selo também aparece em "Faturas do mês" no Painel.
  - **Resumo no topo:** saldo em contas, faturas do mês, dívida total nos cartões e limite disponível.
  - **No cartão:** **fatura do mês** (o que entrou no mês, a mesma conta do Painel, com o vencimento e a situação da fatura) separada da **dívida total** (com parcelas futuras), com ⓘ do glossário; uso do limite (ou "acima do limite em R$ X") e **melhor dia de compra**.
  - **Histórico de faturas** recolhido no próprio cartão, com situação (paga, fechada, em aberto) e "mostrar mais antigas". `/contas?faturas=ID` abre o histórico daquele cartão.
  - Atalho para as transações de cada conta e cartão. Contas do mesmo banco ficam juntas.
  - Colunas novas `Conta.Banco` e `Conta.ContaVinculadaId` (rodar `scripts/2-criar-banco.sh`).
- Transações: filtro **Descrição pouco informativa** (#9), com ⓘ do glossário. Entram as descrições que não dizem o que foi ("PIX ENVIADO", "PAGAMENTO", "TED 123456", "COMPRA CARTAO DEB", só o nome do banco): tirando números soltos, pontuação e palavras genéricas (tipo de operação, bancos, conectivos), sobram menos letras que o mínimo em Parametrização › Transações (padrão 3). Ficam de fora as de categoria escolhida à mão e as marcadas como revisadas. Na URL, `?generica=1`, com chip; cada linha tem "marcar como revisada" (com desfazer) e a barra de seleção marca várias. O Painel mostra um aviso discreto (sem ⚠) com quantas são nos últimos 90 dias, um exemplo e "Revisar". Funciona junto com "Editar categorias" (#87). Coluna nova `Transacao.DescricaoRevisada` (rodar `scripts/2-criar-banco.sh`).
- Transações: **modo edição de categorias** (#87, ADR 0023). "Editar categorias" (fica na URL, `?editar=1`) troca a categoria direto na lista: a troca fica pendente (linha marcada "alterada") até "Salvar alterações (N)", com "Descartar" e Esc para desfazer uma linha; trocar de página ou de filtro com pendências pergunta antes. A barra de seleção marca a troca nas selecionadas, e "Todas as N do filtro" troca de uma vez, inclusive as das outras páginas (com confirmação). Depois de salvar, o aviso tem **Desfazer** (10 min; a API guarda o estado anterior e só aceita a categoria) e, se várias transações iguais foram para a mesma categoria, a tela oferece criar a regra. No celular, cada transação vira um bloco com o seletor em largura total. API: `PATCH /api/transacoes/em-lote` e `POST /api/transacoes/em-lote/desfazer/{chave}`.
- Avisos com ação (ex.: "Desfazer") ficam 10 s e não somem enquanto o mouse ou o foco estão neles.
- **Erros do sistema** em Configurações › Erros (#11, ADR 0022). Avisos e erros da API, da sincronização do Open Finance, das cotações e do câmbio, e falhas da tela no navegador, gravados no banco sem dados sensíveis: query strings, tokens, senhas, CPF e valores em R$ viram `[removido]`. Lista com filtros por situação, período, origem e nível (na URL), "N vezes desde…" para repetições, detalhe com a pilha sob demanda, "Resolvido"/"Reabrir" e "marcar todos do filtro como resolvidos". O Painel avisa quando há erros em aberto na última semana. Os mais antigos que o prazo em Parametrização › Erros do sistema (padrão 90 dias) são apagados sozinhos. Tabela nova `ErroSistema` (rodar `scripts/2-criar-banco.sh`).
- Contas a pagar: **data limite** (#69). Campo "Até (opcional)" nas contas mensais e anuais, ou "quantos pagamentos faltam", que calcula a data. Ao pagar o último, a conta é **encerrada** sozinha: sai de "Próximos vencimentos" no Painel e fica na lista como "encerrada em dd/MM/aaaa". A lista mostra "até dez/2026 · faltam 3" (ou "último pagamento"), e o pagamento do último avisa que a conta será encerrada. Data limite antes do próximo vencimento é recusada pela API e apontada no formulário. Sem data limite, nada muda. Coluna nova `ContaAgendada.DataLimite` (rodar `scripts/2-criar-banco.sh`).
- Metas: ⓘ **Como calculei** no topo do assistente (#90): a conta da recomendação e o aviso de que dinheiro guardado lançado como despesa diminui a sobra prevista.
- Metas: **assistente de quanto guardar** (#90). No topo, a sobra prevista do mês (a mesma da Projeção, com as parcelas já descontadas) menos uma margem de segurança (padrão 10%, em Configurações › Parametrização › Metas) e quanto guardar no total; em cada meta, "Este mês: guarde R$ X", a divisão igual como referência e um **plano mês a mês** (12 meses, aberto sob demanda). As metas têm **prioridade** (↑/↓ no cartão, `PUT /api/metas/prioridades`): metas com prazo recebem primeiro o necessário para cumpri-lo, a sobra vai às sem prazo na ordem de prioridade; quando não dá para todas, vale a prioridade e a meta fora do prazo mostra a data provável e quanto os próximos meses precisam para mantê-lo. Mês que fecha no vermelho recomenda R$ 0 e aponta para a Projeção; com menos de 3 meses de extrato, fica só a divisão igual (`GET /api/metas/plano`). Coluna nova `Meta.Prioridade` (rodar `scripts/2-criar-banco.sh`).
- Metas: acompanhar um **cofrinho** (Reserva do Mercado Pago), não só a conta (#89). Ao escolher uma conta com cofrinhos, aparece "Qual cofrinho (reserva)?" com o nome e o saldo de cada um; o progresso passa a ser o valor inicial + o saldo do cofrinho, sem somar o disponível da conta. Saldo só estimado mostra "confira em Reservas" (`/investimentos?aba=reservas`, que agora abre direto na aba); cofrinho que sumiu do extrato vira aviso com "Editar a meta", sem apagar o vínculo. Coluna nova `Meta.ReservaNome` (rodar `scripts/2-criar-banco.sh`).
- Painel em duas seções (UX-12, #88): **Situação hoje** (posição de hoje, não muda com o mês) e **Mês de …** (o seletor de mês fica nela). "Faturas de cartão" passa a se chamar **Dívida total nos cartões**, com ⓘ do glossário (inclui parcelas futuras; não é a fatura do mês), e o mês ganha o cartão **Faturas do mês**: por cartão, só as transações lançadas no mês escolhido (compras e a parcela do mês; parcelas futuras e pagamento de fatura ficam de fora), quanto disso é parcela e, como referência, a fatura que vence no mês e a situação dela (paga, fechada ou em aberto), com link que abre as faturas do cartão em Contas (`/contas?faturas=ID`).
- Orçamentos: **✨ Sugerir orçamentos** (#39). Para cada categoria, sugere um valor a partir da mediana dos últimos 6 meses (sem emergências, compras parceladas e picos isolados), com o motivo de cada valor. A soma cabe em renda média − parcelas (cartão, financiamentos e empréstimos) − meta de poupança: corta primeiro o estilo de vida, depois o que está sem classificação, e nunca os essenciais sozinho (a tela diz quanto falta). Com os cartões acima do nível de atenção de uso do limite, aperta o estilo de vida pago no cartão. Tudo é revisado numa janela com valores editáveis, total do mês e distribuição da renda (50/30/20) atualizados na hora; categoria que já tem orçamento vem desmarcada. Os percentuais ficam em Configurações › Parametrização › Orçamentos.
- Transações: filtro de **várias categorias** de uma vez, com caixas de marcar (cada categoria inclui as subcategorias, e dá para somar "Sem categoria"). Um chip por categoria em "Filtrando:", cada um com o seu ✕; na URL, `?categoria=12,20,sem` (links antigos com uma categoria continuam valendo). A API aceita `categoriaIds` repetido, mantendo `categoriaId`.
- Empréstimos e financiamentos importados pelo Open Finance (`GET /loans` da Pluggy): saldo devedor, parcelas, taxa, CET e histórico de pagamentos de cada contrato.
- Nova página **Empréstimos e financiamentos**, com financiamentos (imóvel, veículo, bens) e empréstimos sempre separados: totais, contratos e pagamentos por mês.
- Painel: patrimônio líquido passa a descontar o saldo devedor de financiamentos e de empréstimos (cada um em seu cartão), com alerta de parcelas em atraso.
- Câmbio: cotação PTAX oficial do Banco Central (`GET /api/cambio/moedas` e `GET /api/cambio/cotacao?moeda=USD&data=2026-10-15`), guardada no banco para uso offline (`GET /api/cambio/cotacoes`) e atualizada automaticamente uma vez por dia e de novo depois do fechamento do Banco Central (~13h30).
- Investimentos em moeda estrangeira (Open Finance) entram no patrimônio, na reserva e nos totais da carteira convertidos para reais; a tela de investimentos mostra o valor original e o convertido. Moeda sem cotação fica fora dos totais, com aviso.
- Ativos no exterior na carteira (ex.: AAPL, VOO): em **+ Lançar compra/venda**, escolha a moeda; o câmbio vem preenchido com a PTAX do dia e pode ser trocado pelo da corretora. Custo e resultado em reais por operação, valor atual pelo câmbio de hoje e proventos pelo câmbio da data-com.
- Visão anual: o comprometimento da renda soma as parcelas de cartão, financiamentos e empréstimos, mostradas separadamente.
- Configurações em abas (**Open Finance**, **Parametrização**, **Segurança**), com link direto por `?aba=`.
- Nova aba **Parametrização**: sincronização automática, intervalo, dias relidos e histórico da primeira carga; dias de próximos vencimentos e de aviso de consentimento no painel; tentativas de login e tempo de bloqueio. Cada parâmetro mostra a faixa válida e o padrão, vale na hora (sem reiniciar) e é validado pela API (`GET/PUT /api/parametros`). Bloco **Sistema** somente leitura com versão, caminhos, tamanho do banco e totais (`GET /api/parametros/sistema`).
- Tentativas de login e tempo de bloqueio saem do `appsettings.json` para a Parametrização; o valor do `appsettings.json`, se existir, vira o padrão.
- Reservas do Mercado Pago ("cofrinhos"), cujo saldo não vem pelo Open Finance: estimadas pelos lançamentos "Dinheiro reservado/retirado" do extrato, com conferência opcional do saldo real na nova aba **Investimentos › Reservas**. Entram no total investido, no patrimônio e na reserva de emergência. Reservas que precisam ser conferidas (possível saldo anterior à janela importada ou conferência com mais de 30 dias) são destacadas e geram alerta no Painel (#25, ADR 0010).
- Nova aba **Aparência**: paletas prontas (Lavanda, Oceano, Floresta, Grafite, Pôr do sol), cada uma com versão clara e escura, ou uma cor personalizada (tons calculados para manter contraste de 4,5:1), com prévia. Verde, vermelho, laranja de emergência e as cores de financiamento/empréstimo não mudam. Fica salva no navegador.
- Ícones do [Lucide](https://lucide.dev) (licença ISC, empacotados no build) no menu, nos botões de sincronizar, ocultar valores e tema, e nos cartões do painel.
- Novo logo e ícone do site (cofrinho), com versões PNG para atalho no celular e na área de trabalho e `manifest.webmanifest`.
- **Contas a pagar** sugere cadastrar os pagamentos que se repetem todo mês no extrato (boleto, débito automático, Pix), com vencimento estimado pelo dia habitual e valor pela mediana; cada sugestão pode ser cadastrada (formulário preenchido), conferida em Transações ou ignorada (`GET /api/contas-agendadas/sugestoes`, #62).

- Script para parar tudo o que o projeto deixou rodando (serviço, API fora do serviço, `ng serve`) e conferir se o banco ficou livre: `parar-windows.cmd` no Windows, com dois cliques e sem bash, e `scripts/parar.sh` no Linux/macOS. `--desativar` tira o serviço da inicialização automática. Só procura processos com a pasta do projeto na linha de comando (não derruba outro app nem um `dotnet build`) e sai com código 1 se algo ficou rodando. Os testes de instalação no Windows e no Linux param e sobem o sistema de novo.

### Correções (layout)
- Abas no celular (ex.: Configurações): as que não cabem descem de linha, em vez de sair da tela.
- Mensagem de API fora do ar falava em IIS; agora pede para conferir o serviço Financas Local.
- Contas a pagar no celular: a página ficava mais larga que a tela e o formulário abria cortado à direita. O texto do cabeçalho só para leitor de tela escapava da tabela com rolagem; agora fica preso nela (vale para todas as tabelas com rolagem).
- Avisos de lista vazia (ex.: "Nenhum empréstimo ou financiamento encontrado.") ficavam fora do meio do cartão em telas largas: o texto era centralizado dentro de uma caixa de 78 caracteres encostada à esquerda. Agora a caixa também é centralizada, em todas as telas (#46).
- Contraste da cor principal da paleta Lavanda (texto dos botões) passa de 3,5:1 para 4,7:1.
- Arquivos estáticos sem hash no nome (`favicon.svg`, ícones, manifesto) deixam de ficar em cache por um ano: antes, uma troca de ícone não aparecia no navegador.

### Banco
- Novas tabelas `Emprestimo` e `PagamentoEmprestimo` em `banco/01-estrutura.sql`: rode `scripts/2-criar-banco.sh` antes de publicar.
- Nova tabela `CotacaoMoeda` (cache das cotações de câmbio) em `banco/01-estrutura.sql`.
- Novas colunas `Ativo.Moeda` e `OperacaoAtivo.Cambio`: rode `scripts/2-criar-banco.sh` (primeiro uso de `adicionar_coluna_se_ausente`).
- Nova tabela `ReservaConferencia` (saldo conferido das reservas) em `banco/01-estrutura.sql`: rode `scripts/2-criar-banco.sh`.

### Correções
- Relatórios: **transferência para outra pessoa conta como despesa ou receita** (ADR 0031). Antes, todo Pix ou TED na categoria "Transferências" ficava fora dos relatórios, inclusive salário e pagamentos a terceiros.
  - **Continua fora só o que é entre contas suas:** o par aparece na outra conta (mesmo valor, sinal oposto, até 5 dias), o banco informa o mesmo titular, a transferência foi lançada pelo site ou é dinheiro guardado numa reserva.
  - **Quando roda:** na sincronização, ao iniciar o site e depois de mudar categorias.
  - **Outras categorias do tipo transferência** (Investimentos, Pagamento de fatura e as criadas por você) continuam sempre fora.
- Investimentos e Open Finance: **compra lançada à mão não trava mais a importação.** Antes, se você lançasse a compra que faltou no histórico da corretora, aquele ativo parava de receber as próximas compras e vendas do Open Finance. Agora, num ativo que já tem histórico da corretora, o seu lançamento só completa o que faltou. Se a corretora mandar depois a mesma operação (mesmo tipo e quantidade, até 5 dias de diferença), as duas são ligadas, sem duplicar. Ativos só com lançamentos seus continuam fora da importação, como antes. Também deixou de acontecer: uma falha da Pluggy ao listar as movimentações de um ativo com histórico registrava a posição inteira como uma compra a mais.
- Investimentos: **FII do Nubank entrava como ação.** Pelo Open Finance, o Nubank manda FII com o subtipo "STOCK", e o ativo novo virava ação, ficando fora do total e da aba de FIIs. Agora o emissor ("... FDO INV IMOB - FII") ou o nome oficial do fundo, que chega com a primeira cotação, faz o ativo virar FII. Os que já entraram errado são corrigidos por "Conferir investimentos".
- Janelas sobrepostas (ex.: editar uma operação a partir da lista de operações): a aberta por último fica por cima, e o Esc fecha só ela — antes, a edição abria atrás e um Esc fechava todas.
- Investimentos: a tela não carregava (erro 500) quando o Banco Central ou outra fonte demorava mais que o tempo limite; agora segue sem o dado, como já fazia sem internet. Mesmo ajuste nas cotações, no câmbio, na importação pelo extrato e na sincronização do Open Finance.
- Publicação no Linux: `scripts/3-publicar.sh` passa a se chamar `scripts/3-publicar-atualizar.sh` e reinicia o serviço quando ele já está rodando (antes, continuava com a versão antiga em memória).
- Instalação num Windows novo: o `1-preparar-ambiente.sh` deixa de depender de Python (que no Windows costuma ser só o atalho da Microsoft Store) e de `unzip`, baixa o `sqlite3` portátil oficial quando falta, confere as ferramentas do terminal antes de começar e mostra as versões instaladas no final. O `3-publicar-atualizar.sh` também não usa mais Python, funciona com `--pular-build` sem o .NET no PATH e avisa logo no início quando o Git Bash não está como Administrador (ADR 0011).
- Instalação em um passo: `scripts/instalar.sh` roda as três etapas em sequência. No Windows, basta dar dois cliques em `instalar-windows.cmd`, que pede Administrador e roda `scripts/instalar-windows.ps1` — só PowerShell, sem Git, bash nem Python (ADR 0012).
- Publicação no Windows: o serviço é parado antes de copiar os arquivos novos (antes, a cópia falhava com as DLLs em uso).
- Instalador do Windows: o log guarda também as mensagens de erro do npm/dotnet e os eventos do Windows quando a API não sobe; o erro final diz em que etapa parou; API que não responde em 30 s reprova a instalação (antes, terminava em verde); certificado confiável para todas as contas do computador, sem janela de confirmação; clique na janela não congela mais a instalação (QuickEdit desligado); no fim, abre o site e fecha sozinho (só fica aberto se der erro); o log acumula as execuções.
- Atualização mais segura (Windows e bash): cópia do banco em `backups/FinancasLocal-antes-instalacao-*.db` antes de mudar a estrutura (ficam as 5 mais recentes), `sqlite3` espera em vez de falhar com `database is locked` se a API estiver gravando; no Windows, se a versão nova não subir, a anterior volta sozinha.
- Backup e criação do banco funcionam em pastas com apóstrofo no caminho (ex.: `C:\Users\D'Avila`).
- Backup agendado no Windows (`4-backup.sh --agendar`): a tarefa era criada com caminhos do Git Bash (`/usr/bin/bash`, `/c/...`), que o Agendador de Tarefas não entende, e não rodava. Agora usa caminhos do Windows e o `bash.exe` do Git for Windows.
- Validação do código da B3 volta ao formato estrito (ex.: PETR4, MXRF11); ativos de fora da B3, como AOK, entram escolhendo a moeda (ADR 0009).

### Documentação
- README: **instalar pelo `.msi`** (passo a passo, aviso do SmartScreen, conferir o SHA256, o que fica onde, o gerenciador), **atualizar o site** (pelo gerenciador, pelo `.msi` novo, saindo da instalação por scripts para o instalador, ou continuando pelos scripts), **desinstalar** e problemas comuns do instalador. A carteira de renda variável passa a explicar o lançamento manual depois do #117 e o "Conferir investimentos".
- Pasta `docs/` restaurada (fora apagada por engano na migração para SQLite) e atualizada para SQLite/Kestrel e scripts bash.
- Decisões de arquitetura passam a ser registradas como ADRs em `docs/adr/`, com as decisões já tomadas registradas retroativamente.

### Infraestrutura
- Testes automatizados do front-end com **Jest** (`npm test` em `src/frontend`): 84 testes de regressão (filtros de Transações na URL, balão do glossário dentro da tela, utilidades de datas e links) e de usabilidade (nome e estado acessível, teclado e Esc, "Mais análises" que só cria o conteúdo ao abrir, filtros ativos visíveis, status da sincronização anunciado). Rodam na CI (ADR 0013).
- Novo workflow **Teste de instalação no Linux**: roda `scripts/instalar.sh` num runner Ubuntu sem dotnet, node, sqlite3 e Python no PATH, confere o serviço `systemd --user` e a API, testa a atualização e o `4-backup.sh`.
- Novo workflow **Teste de instalação no Windows**: instala do zero num runner Windows (sem as ferramentas do runner no PATH), confere o serviço e a API, e testa a atualização pelo `instalar-windows.cmd`.
- Backend migrado de SQL Server Express/IIS para SQLite/Kestrel — o projeto agora roda em Windows e Linux com os mesmos scripts.
- Nova ferramenta `Financas.MigrarSqlite` para migrar os dados de uma instalação antiga com SQL Server para o SQLite novo.
- Scripts `.sh` (bash) substituindo os `.ps1` de instalação, criação do banco, publicação e backup.

## [1.0.0] — 2026-10-15

Primeira versão versionada no Git.

### Funcionalidades
- Painel com patrimônio líquido (com e sem renda variável), saldos, faturas, receitas × despesas, gastos por categoria, orçamentos, vencimentos e alertas.
- Open Finance via Pluggy/Meu Pluggy: contas, cartões, faturas, transações (`/v2/transactions`) e investimentos, com sincronização agendada.
- Transações com filtros, categorização em lote, regras automáticas, sugestões de categoria pelo histórico e marcação de gastos de emergência.
- Contas e cartões (limite efetivo, faturas), orçamentos com destaque laranja para estouro por emergência, metas, contas a pagar.
- Relatórios: fluxo mensal com emergências, categorias, maiores despesas, projeção mês a mês e visão anual com indicadores de saúde financeira.
- Investimentos: carteira de renda variável (preço médio, proventos, DY, yield on cost), comparação com a Selic, aba de FIIs com gráficos, importação pelo Open Finance e pelo extrato.
- Calculadora flutuante, modo escuro, opção de ocultar valores.

### Infraestrutura
- API .NET 10 no IIS (in-process), Angular 20, SQL Server 2025 Express local.
- Scripts de instalação, criação do banco (sem migrations), publicação e backup.
