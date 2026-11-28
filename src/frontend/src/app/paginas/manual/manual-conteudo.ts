import { ChaveGlossario } from '../../compartilhado/glossario/glossario';

/**
 * Manual do site (#113, ADR 0026). Conteúdo em dados, num lugar só: a página de Manual desenha, a busca procura e
 * os testes conferem que todo link "Abrir a tela" aponta para uma rota que existe e que toda tela do menu tem seção.
 * Mudou o jeito de usar uma tela? Atualize a seção aqui (checklist do PR em CONTRIBUINDO.md).
 *
 * Linguagem: a de quem usa (Nielsen 2), frases curtas, voz ativa (#733). Nomes de botões e campos entre aspas curvas,
 * exatamente como aparecem na tela. Cada seção é objetiva: uma linha, o print, até 4 tópicos e o resto em "Mais
 * detalhes", fechado (divulgação progressiva, ADR 0071).
 */
export interface LinkManual {
  rotulo: string;
  /** Rota do site (ex.: '/transacoes'). */
  rota: string;
  params?: Record<string, string>;
}

export interface SecaoManual {
  /** Âncora da seção (/manual/<capitulo>#<id>). */
  id: string;
  titulo: string;
  /** Para que serve, em uma frase. */
  paraQueServe: string;
  /** Print da tela (ADR 0071): `imagens/manual/<id>-claro.webp` e `-escuro.webp`, gerados por `npm run prints:manual`. Complementa o texto, nunca o substitui. */
  print?: { id: string; alt: string };
  /** De 1 a 4 tópicos curtos, em lista sem numeração (o quê → onde). */
  topicos: string[];
  /** Só quando é passo a passo de verdade e a ordem importa: lista numerada, até 6 itens curtos. */
  passos?: string[];
  /** "Abrir a tela": leva direto, já filtrada quando faz sentido (detalhe por navegação). */
  abrir?: LinkManual[];
  /** O resto, em "Mais detalhes" (fechado): teclado, celular, casos raros, o que antes eram "Dicas" e "Erros comuns". A busca também procura aqui. */
  detalhes?: string[];
  /** Termos do glossário explicados com ⓘ nesta seção. */
  termos?: ChaveGlossario[];
}

export interface CapituloManual {
  /** /manual/<id> */
  id: string;
  titulo: string;
  /** Uma linha para o índice. */
  resumo: string;
  secoes: SecaoManual[];
}

export const MANUAL: CapituloManual[] = [
  {
    id: 'primeiros-passos',
    titulo: 'Primeiros passos',
    resumo: 'O que é o Finanças Local, o primeiro acesso, onde fica cada coisa e por onde começar.',
    secoes: [
      {
        id: 'o-que-e',
        titulo: 'O que é o Finanças Local',
        paraQueServe: 'Um controle financeiro pessoal que roda no seu computador: seus dados ficam com você, não num serviço na internet.',
        topicos: [
          'O site abre no navegador, mas quem responde é um programa instalado no seu computador.',
          'Os dados ficam num arquivo, na pasta “dados” da instalação.',
          'Os extratos chegam pelo Open Finance (Meu Pluggy), só se você configurar.',
          'Para a internet saem só cotações e taxas; valores e dados pessoais não saem.'
        ],
        abrir: [{ rotulo: 'Abrir o Painel', rota: '/painel' }],
        detalhes: [
          'O endereço começa com https://localhost ou https://127.0.0.1. No celular, com o acesso pelo celular ligado, é o IP do computador na rede de casa.',
          'Saem para a internet: pedidos de cotação e de taxas (códigos de ativos e de moedas e datas), a conversa com a Pluggy e, no gerenciador, a procura de versão nova no GitHub.',
          'Veja o capítulo “Privacidade, backup e atualização” para saber exatamente o que sai do computador.'
        ]
      },
      {
        id: 'primeiro-acesso',
        titulo: 'Primeiro acesso, entrar e sair',
        paraQueServe: 'Criar o seu usuário e proteger o acesso ao site.',
        topicos: [
          'Na primeira vez, crie um usuário e uma senha (8 caracteres ou mais).',
          'Depois, entre com eles. “Sair” (menu do usuário, no canto de cima) encerra o acesso em todos os aparelhos.',
          'A sessão cai depois de 2 h sem uso ou 12 h depois de entrar; é só entrar de novo.'
        ],
        abrir: [{ rotulo: 'Trocar a senha', rota: '/configuracoes', params: { aba: 'seguranca' } }],
        detalhes: [
          'Muitas senhas erradas seguidas bloqueiam o login por alguns minutos. Espere e tente de novo; o limite e o tempo ficam em Configurações › Parametrização.'
        ]
      },
      {
        id: 'conhecendo-a-tela',
        titulo: 'Conhecendo a tela',
        paraQueServe: 'Saber onde ficam o menu, o status da sincronização e as ferramentas que valem em qualquer tela.',
        print: { id: 'conhecendo-a-tela', alt: 'Menu lateral com os grupos de telas, a barra de cima com Ajuda, avisos e status da sincronização, e o começo do Painel.' },
        topicos: [
          'Menu à esquerda → as telas, em grupos por assunto; clique no nome do grupo para abrir ou fechar.',
          'Barra de cima → status da sincronização (clique para sincronizar), sino de avisos e “Ajuda”.',
          'Olho → esconde os valores; calculadora → Alt+C; menu do usuário → tema, cores e “Sair”.',
          'Palavras com ⓘ → clique para ver a explicação.'
        ],
        detalhes: [
          'Grupos do menu: o Painel no topo; Movimentação (Transações, Contas e cartões, Contas a pagar); Planejamento (Orçamentos, Metas); Patrimônio (Investimentos, Empréstimos e financiamentos); Análises (Relatórios, Visão anual, Projeção); Ajustes (Categorias e regras, Configurações); e o Manual no rodapé.',
          'Os grupos lembram se estavam abertos, neste navegador. Ao entrar numa tela, o grupo dela abre sozinho; se você o fechar, o nome fica destacado, com um ponto.',
          'No celular, o menu abre pelo botão ☰. Tocar numa tela leva até ela e fecha o menu; para fechar sem escolher, toque no ✕, fora do menu, ou arraste-o para a esquerda.',
          'No computador, o botão ☰ esconde o menu para sobrar espaço para tabelas e gráficos. Clicar em “Finanças” leva ao Painel, de qualquer tela.',
          'Clicar em “Atualizado há…” sincroniza na hora. O aviso diz quantas transações são novas e quantas mudaram, e “Ver em Transações” abre só elas. “1 conexão com erro” leva às Configurações.',
          'O sino junta as notificações de qualquer tela: orçamentos estourados e avisos (conta vencida, consentimento vencendo, falha de sincronização, erros). O número conta o que está pendente.',
          'Campos de data: digite dd/mm/aaaa (as barras entram sozinhas) ou abra o calendário. ‹ › trocam o mês e « » o ano; clique no título para escolher o mês e, de novo, o ano. Esc volta um passo.'
        ]
      },
      {
        id: 'por-onde-comecar',
        titulo: 'Por onde começar',
        paraQueServe: 'Uma ordem que deixa os relatórios certos o quanto antes.',
        topicos: ['Dados primeiro (bancos e contas), depois organização (categorias e regras) e, por fim, planejamento.'],
        passos: [
          'Conecte seus bancos pelo Open Finance (capítulo “Conectar seus bancos”).',
          'Confira as contas e cartões: o banco de cada um e a conta de cada cartão.',
          'Categorize as transações do mês, começando pelas “Sem categoria”.',
          'Crie regras para o que se repete: as próximas já vêm categorizadas.',
          'Monte os orçamentos (dá para pedir uma sugestão).',
          'Crie suas metas e veja quanto guardar por mês.'
        ],
        abrir: [
          { rotulo: 'Conectar os bancos', rota: '/configuracoes' },
          { rotulo: 'Ver as transações sem categoria', rota: '/transacoes', params: { categoria: 'sem' } }
        ]
      }
    ]
  },
  {
    id: 'corrigir-seus-dados',
    titulo: 'Corrigir seus dados',
    resumo: 'Depois de atualizar o site: o que já se acertou sozinho e o passo a passo para deixar tudo igual aos apps dos bancos.',
    secoes: [
      {
        id: 'o-que-ja-se-acertou',
        titulo: 'O que já se acertou sozinho',
        paraQueServe: 'Saber o que não precisa ser refeito à mão depois da atualização.',
        topicos: ['Estorno no cartão (compra devolvida, cashback) deixou de contar como receita e passou a abater o gasto da…'],
        detalhes: [
          'Estorno no cartão (compra devolvida, cashback) deixou de contar como receita e passou a abater o gasto da categoria.',
          'Pix e TED para outra pessoa contam como despesa ou receita; os que são entre contas suas ficam fora dos gastos.',
          'Pagamento de fatura que o banco mandou como boleto ou “outros” saiu dos gastos quando o crédito do mesmo valor aparece no cartão.',
          'Aplicação e resgate de RDB, CDB e poupança, e a compra e a venda de Méli Dólar, foram para “Investimentos” e não contam mais como gasto nem receita.',
          'A projeção não conta mais as parcelas em dobro, e a fatura do mês do cartão segue o ciclo do banco.',
          'O lançamento pendente que o banco efetivou com outro código sai na próxima sincronização.',
          'O que você categorizou à mão não foi mexido: as correções só valem para o que veio automático.'
        ],
        abrir: [{ rotulo: 'Ver as transações', rota: '/transacoes' }]
      },
      {
        id: 'passo-1-sincronizar',
        titulo: '1. Sincronize os bancos',
        paraQueServe: 'Trazer os dados novos que as correções usam: o valor reservado que o banco informa e as pendências que já viraram compra.',
        topicos: ['Abra Configurações › Open Finance e clique em “Sincronizar agora”.'],
        detalhes: [
          'Abra Configurações › Open Finance e clique em “Sincronizar agora”.',
          'Espere a barra de cima mostrar “Atualizado agora”.'
        ],
        abrir: [{ rotulo: 'Abrir Configurações › Open Finance', rota: '/configuracoes' }]
      },
      {
        id: 'passo-2-reservas',
        titulo: '2. Confira as reservas, as caixinhas e o Méli Dólar',
        paraQueServe: 'Fazer o dinheiro guardado entrar certo no patrimônio.',
        topicos: ['Abra Investimentos, na aba Reservas.'],
        detalhes: [
          'Abra Investimentos, na aba Reservas.',
          'Reserva com “informado pelo banco”: já está certa, não precisa fazer nada. A linha “Reservado” com “restante do total do banco” também não se confere: ela se ajusta quando você confere as outras.',
          'Reserva “estimada” ou “para conferir”: clique em “conferir saldo” e digite o valor que o app do banco mostra, do jeito que aparece lá (ex.: 10.123,42).',
          'Méli Dólar: em “conferir saldo”, escolha se vai digitar em reais ou em dólares, como o app do Mercado Pago mostra.',
          'Confira depois o patrimônio no Painel: ele passa a somar essas reservas.'
        ],
        abrir: [{ rotulo: 'Abrir as reservas', rota: '/investimentos', params: { aba: 'reservas' } }],
        termos: ['ptax']
      },
      {
        id: 'passo-3-trocos',
        titulo: '3. Tire os trocos das compras de FIIs dos rendimentos',
        paraQueServe: 'Fazer os rendimentos dos FIIs baterem com o app da corretora.',
        topicos: ['Se o Painel avisa “N créditos de troco ou venda de FIIs e ações contando como rendimento”, clique em…'],
        detalhes: [
          'Se o Painel avisa “N créditos de troco ou venda de FIIs e ações contando como rendimento”, clique em “Conferir investimentos” no aviso (ou no botão de Investimentos).',
          'Revise a lista “Troco ou venda contando como rendimento” e clique em “Corrigir selecionados”.',
          'Nada muda sem você confirmar, e o aviso tem “Desfazer” por 10 minutos.'
        ],
        abrir: [{ rotulo: 'Abrir Conferir investimentos', rota: '/investimentos', params: { conferir: '1' } }],
        termos: ['trocoDeCompra']
      },
      {
        id: 'passo-4-limite',
        titulo: '4. Responda a pergunta do limite do cartão',
        paraQueServe: 'Não tratar como dívida o limite que você tirou do cartão no app.',
        topicos: ['Abra Contas e cartões.'],
        detalhes: [
          'Abra Contas e cartões.',
          'O cartão em que você reduziu o limite no app pergunta “Você reduziu o limite no app?”. Clique em “responder” e em “Sim, ajustei o limite”.',
          'Se a diferença for compra em processamento ou parcela, responda “Não”.'
        ],
        abrir: [{ rotulo: 'Abrir Contas e cartões', rota: '/contas' }],
        termos: ['limiteUsado']
      },
      {
        id: 'passo-5-nome',
        titulo: '5. Informe o seu nome como aparece nos Pix',
        paraQueServe: 'Separar o Pix para outra conta sua do Pix para outra pessoa, quando o banco não informa.',
        topicos: ['Abra Configurações › Open Finance.'],
        detalhes: [
          'Abra Configurações › Open Finance.',
          'Em “Seu nome como aparece nos Pix”, digite o primeiro e o último nome (ex.: como aparece no comprovante) e clique em “Salvar nome”.',
          'As transferências são revistas na hora.'
        ],
        abrir: [{ rotulo: 'Abrir Configurações › Open Finance', rota: '/configuracoes' }]
      },
      {
        id: 'passo-6-assinaturas',
        titulo: '6. Cadastre as assinaturas e os débitos automáticos',
        paraQueServe: 'Saber o que cobra em cada cartão e conta todo mês, e ser avisado quando algo muda.',
        topicos: ['Abra Contas a pagar e veja as “Sugestões do extrato”: assinaturas nos cartões, débitos automáticos e contas…'],
        detalhes: [
          'Abra Contas a pagar e veja as “Sugestões do extrato”: assinaturas nos cartões, débitos automáticos e contas que se repetem.',
          'Clique em “Cadastrar” nas que são suas; a forma de cobrança e o texto já vêm preenchidos. “ignorar” tira a sugestão.',
          'Na aba “Assinaturas e recorrentes”, confira onde cada uma cai e quanto vai por mês.'
        ],
        abrir: [
          { rotulo: 'Abrir Contas a pagar', rota: '/contas-a-pagar' },
          { rotulo: 'Abrir Assinaturas e recorrentes', rota: '/contas-a-pagar', params: { aba: 'recorrentes' } }
        ]
      },
      {
        id: 'passo-7-conferir',
        titulo: '7. Confira com os apps dos bancos',
        paraQueServe: 'Ter certeza de que o site bate com cada banco, e saber por que quando não bate.',
        topicos: ['Abra Contas e cartões e clique em “Conferir com o banco”.'],
        detalhes: [
          'Abra Contas e cartões e clique em “Conferir com o banco”.',
          'Com o app de cada banco aberto, digite o saldo de cada conta, a fatura do mês de cada cartão e o valor de cada investimento e reserva.',
          '“bate”: está certo. Quando não bate, a pista com o mesmo valor da diferença vem primeiro, marcada “Explica a diferença”, com o link “ver” para corrigir.',
          'Fatura diferente do app? Veja se o app mostra a dívida total (com as parcelas futuras) ou compras que ainda estão pendentes: as duas aparecem como pista.',
          'Faça isso logo depois de sincronizar: o banco atualiza o Open Finance mais ou menos uma vez por dia.'
        ],
        abrir: [{ rotulo: 'Conferir com o banco', rota: '/conferir-banco' }]
      }
    ]
  },
  {
    id: 'conectar-bancos',
    titulo: 'Conectar seus bancos (Open Finance)',
    resumo: 'Meu Pluggy, credenciais, conexões, sincronização automática e o que fazer quando dá erro.',
    secoes: [
      {
        id: 'meu-pluggy',
        titulo: 'Autorizar seus bancos no Meu Pluggy',
        paraQueServe: 'O Open Finance precisa do seu consentimento em cada banco; o Meu Pluggy é o caminho para isso.',
        topicos: ['Crie sua conta em meu.pluggy.ai.'],
        detalhes: [
          'Crie sua conta em meu.pluggy.ai.',
          'Conecte cada banco pelo fluxo oficial do Open Finance: o consentimento é dado no aplicativo do próprio banco.',
          'O consentimento vale por até 12 meses.',
          'O Meu Pluggy atualiza os dados dos bancos mais ou menos uma vez por dia.'
        ]
      },
      {
        id: 'credenciais',
        titulo: 'Client ID e Client Secret',
        paraQueServe: 'As credenciais que deixam o Finanças Local buscar os seus dados na Pluggy.',
        print: { id: 'credenciais', alt: 'Configurações, aba Open Finance: os campos Client ID e Client Secret e os botões Testar conexão e Salvar.' },
        topicos: ['Em dashboard.pluggy.ai, crie uma conta de desenvolvedor.'],
        detalhes: [
          'Em dashboard.pluggy.ai, crie uma conta de desenvolvedor.',
          'Adicione o conector “MeuPluggy” e crie uma aplicação de desenvolvimento.',
          'Copie o Client ID e o Client Secret da aplicação.',
          'No site, abra Configurações › Open Finance, cole os dois e clique em “Salvar”.',
          'Clique em “Testar conexão” para confirmar.',
          'O Client Secret fica guardado criptografado e nunca é mostrado de novo. Para trocar, cole o novo; para manter, deixe o campo vazio.'
        ],
        abrir: [{ rotulo: 'Abrir Configurações › Open Finance', rota: '/configuracoes' }]
      },
      {
        id: 'conexoes',
        titulo: 'Adicionar as conexões (um Item ID por banco)',
        paraQueServe: 'Cada banco autorizado vira uma conexão, e as contas e cartões dele aparecem sozinhos.',
        topicos: ['No dashboard da Pluggy, na aplicação de demonstração, conecte o “MeuPluggy” e autorize — uma vez para cada…'],
        detalhes: [
          'No dashboard da Pluggy, na aplicação de demonstração, conecte o “MeuPluggy” e autorize — uma vez para cada banco.',
          'Cada autorização gera um Item ID (aparece no dashboard).',
          'Em Configurações › Open Finance, cole o Item ID no campo “Adicionar conexão” e clique em “Adicionar”.',
          'Cada conexão aparece com o selo do banco (o mesmo de Contas e cartões); banco que o site não conhece aparece com um ícone de banco.',
          'Cada conexão mostra a situação em português: “Atualizada”, “Desatualizada”, “Erro de login no banco” ou “Precisa de ação sua” (aí é preciso entrar no Meu Pluggy). Os botões “Sincronizar”, “Pausar” e “Remover” ficam na própria linha.',
          'Clique em “Sincronizar agora”. Na primeira vez são buscados os últimos meses de histórico.',
          'Confira em Contas e cartões se as contas e cartões apareceram.',
          'Pelo Meu Pluggy, toda conexão tem o mesmo nome de conector (“MeuPluggy”). O site mostra o banco, tirado das contas da conexão, e o conector ao lado (“via MeuPluggy”).',
          'Quanto histórico a primeira carga busca fica em Configurações › Parametrização › Sincronização.',
          'A conexão aparece com o nome errado? Escolha o banco certo numa das contas dela, em Contas e cartões › editar.'
        ],
        abrir: [{ rotulo: 'Abrir Configurações › Open Finance', rota: '/configuracoes' }, { rotulo: 'Ver as contas', rota: '/contas' }]
      },
      {
        id: 'sincronizacao',
        titulo: 'Sincronização automática',
        paraQueServe: 'Manter as transações em dia sem clicar em nada.',
        topicos: ['A sincronização roda sozinha de tempos em tempos (padrão: a cada 6 horas).'],
        detalhes: [
          'A sincronização roda sozinha de tempos em tempos (padrão: a cada 6 horas).',
          'A cada vez, ela relê os últimos dias (padrão: 30) para pegar o que mudou ou entrou atrasado.',
          'O status aparece na barra de cima: “Atualizado há…”, “Sincronizando…” ou “1 conexão com erro”.',
          'Para sincronizar na hora, clique em “Atualizado há…” na barra de cima ou use “Sincronizar agora” em Configurações › Open Finance.',
          'O aviso do resultado (“Sincronizado: 3 nova(s), 2 atualizada(s).”) tem “Ver em Transações”: abre as Transações só com essas, no período delas, com o filtro “Novas e atualizadas na sincronização” em “Filtrando:”. Atualizada é a que já existia e mudou de verdade nessa sincronização. O aviso de “Adicionar” conexão traz o mesmo botão, com as transações da primeira carga. Sem nada novo, o aviso vem sem o botão. Com mais de 200, o botão abre o período inteiro, porque são muitas para listar uma a uma. O aviso fica 10 segundos e não some enquanto o mouse ou o foco estão nele.',
          'Lançamento pendente que o banco efetivou com outro código sai sozinho, mesmo que seja de antes dos dias relidos: o site acha a efetivada do mesmo valor na mesma conta (até 10 dias depois), para não contar em dobro.',
          'Conta em outra moeda (ex.: em dólar) gera um aviso nas notificações (o sino da barra do topo): o site soma as contas como reais, sem conversão.'
        ],
        abrir: [{ rotulo: 'Ajustar a sincronização', rota: '/configuracoes', params: { aba: 'parametrizacao' } }]
      },
      {
        id: 'consentimento-e-erros',
        titulo: 'Consentimento vencendo e erros de conexão',
        paraQueServe: 'Saber o que fazer quando um banco para de atualizar.',
        topicos: ['As notificações (o sino da barra de cima) avisam quando o consentimento de um banco vai expirar (padrão: 30…'],
        detalhes: [
          'As notificações (o sino da barra de cima) avisam quando o consentimento de um banco vai expirar (padrão: 30 dias antes). Renove no Meu Pluggy.',
          'Se a barra de cima mostrar “1 conexão com erro”, clique nela: ela leva à lista de conexões, com o último erro de cada uma.',
          'Se pedir “ação no Meu Pluggy”, entre no Meu Pluggy e reautorize o banco.',
          'Detalhes técnicos de falhas ficam em Configurações › Erros.',
          'Erro 401 ou 403 ao sincronizar: confira o Client ID e o Client Secret e se o MeuPluggy foi autorizado para aquele banco.'
        ],
        abrir: [{ rotulo: 'Ver as conexões', rota: '/configuracoes' }, { rotulo: 'Ver os erros do sistema', rota: '/configuracoes', params: { aba: 'erros' } }]
      }
    ]
  },
  {
    id: 'contas',
    titulo: 'Contas e cartões',
    resumo: 'Banco de cada conta, cartão ligado à conta, fatura do mês × dívida total e histórico de faturas.',
    secoes: [
      {
        id: 'tipos-de-conta',
        titulo: 'Contas do Open Finance e contas manuais',
        paraQueServe: 'Entender de onde vem o saldo de cada conta e quando criar uma conta à mão.',
        print: { id: 'tipos-de-conta', alt: 'Contas e cartões: resumo de saldo, faturas e dívida, os botões Conferir com o banco e Conta manual, e o início da lista de contas.' },
        topicos: ['Contas do Open Finance aparecem sozinhas depois da sincronização; saldo, limite e tipo vêm do banco.'],
        detalhes: [
          'Contas do Open Finance aparecem sozinhas depois da sincronização; saldo, limite e tipo vêm do banco.',
          'Para dinheiro em espécie, banco sem Open Finance ou um controle separado, use “+ Conta manual”.',
          'Na conta manual você informa o tipo, o saldo inicial e, no cartão, limite, dia de fechamento e de vencimento.',
          'Desmarque “Incluir no saldo total” para uma conta não somar no saldo (ex.: conta de outra pessoa).',
          'Conta que não usa mais: marque “Arquivada” em vez de excluir.',
          'Para ver as arquivadas, ligue o interruptor “Mostrar arquivadas” no topo da tela; elas aparecem com o selo “Arquivada”.'
        ],
        abrir: [
          { rotulo: 'Abrir Contas e cartões', rota: '/contas' },
          { rotulo: 'Ver também as arquivadas', rota: '/contas', params: { arquivadas: '1' } }
        ]
      },
      {
        id: 'banco-da-conta',
        titulo: 'O banco de cada conta (o selo)',
        paraQueServe: 'Reconhecer de relance qual conta e qual cartão é de qual banco.',
        topicos: ['Cada conta mostra o selo do banco, reconhecido pelo nome da conta e da instituição.'],
        detalhes: [
          'Cada conta mostra o selo do banco, reconhecido pelo nome da conta e da instituição.',
          'Se o banco estiver errado (ou não aparecer), clique em “editar” na conta.',
          'No campo “Banco”, escolha o banco certo. “Detectar pelo nome” volta para o reconhecimento automático.',
          'Para não mostrar banco nenhum, escolha “Outro (sem banco)”.',
          'O cartão desenhado fica com a cor do banco; a cor escolhida na conta só vale quando o banco não é identificado.'
        ],
        abrir: [{ rotulo: 'Abrir Contas e cartões', rota: '/contas' }]
      },
      {
        id: 'cartao-e-conta',
        titulo: 'Ligar o cartão à conta dele',
        paraQueServe: 'Mostrar de qual conta o cartão é (e de onde a fatura é paga), e o cartão usar o banco da conta.',
        topicos: ['Clique em “editar” no cartão.'],
        detalhes: [
          'Clique em “editar” no cartão.',
          'Em “Conta do cartão”, escolha a conta do mesmo banco.',
          'Salve. O cartão passa a mostrar “da conta …”, e a conta mostra “Cartões: …”.'
        ],
        abrir: [{ rotulo: 'Abrir Contas e cartões', rota: '/contas' }]
      },
      {
        id: 'fatura-e-divida',
        titulo: 'Fatura do mês × dívida total',
        paraQueServe: 'Não confundir o que entrou no cartão neste mês com tudo o que você deve nele.',
        topicos: ['“Fatura de <mês>” é a fatura do cartão que fecha no mês: as compras desde o fechamento anterior e a parcela…'],
        detalhes: [
          '“Fatura de <mês>” é a fatura do cartão que fecha no mês: as compras desde o fechamento anterior e a parcela das compras parceladas, menos os estornos. Depois de fechada, vale o total que o banco informou.',
          'Abaixo dela aparecem quando a fatura fecha, quando vence e a situação (paga, fechada ou em aberto). Ela aparece como paga quando o banco informa o pagamento ou quando o pagamento (por exemplo, “Pagamento recebido”) está nas transações do cartão, entre o vencimento anterior e o vencimento dela. Pagar antes do fechamento também conta.',
          'Para ver as faturas de outro mês, use o seletor de mês em “Cartões de crédito” (como no Painel). Muda a “Fatura de <mês>” de cada cartão e o total; saldo, dívida total, limite e “Fatura em aberto” continuam sendo os de hoje. “Voltar para o mês atual” volta. O link “Faturas do mês” do Painel abre esta tela no mesmo mês.',
          'Depois que a fatura do mês fecha, aparece embaixo a “Próxima fatura”: o que já entrou na fatura seguinte, que ainda está aberta, com o dia em que ela fecha e vence.',
          '“Dívida total” é tudo o que você deve no cartão hoje, inclusive as parcelas que ainda vão vencer.',
          'A barra mostra quanto do limite está em uso e quanto ainda está disponível (ou quanto passou do limite).',
          '“Melhor dia de compra” é o dia seguinte ao fechamento: o que você compra a partir dele só entra na fatura seguinte.',
          'Ligue “Fatura no Painel” no cartão para somar a fatura em aberto dele no card “Fatura em aberto”, no topo desta tela e no Painel, em “Situação hoje”. “ver detalhes” mostra a fatura de cada cartão. Vale na hora e fica guardado neste navegador: em outro computador ou celular, ligue de novo.'
        ],
        abrir: [{ rotulo: 'Abrir Contas e cartões', rota: '/contas' }],
        termos: ['faturaDoMes', 'dividaCartao', 'melhorDiaCompra']
      },
      {
        id: 'conferir-banco',
        titulo: 'Conferir com o banco',
        paraQueServe: 'Ver se o site bate com o app de cada banco e, quando não bate, por quê.',
        print: { id: 'conferir-banco', alt: 'Conferir com o banco: tabela de contas com o saldo no site, o campo para informar o valor do app do banco e o resultado.' },
        topicos: ['Em Contas e cartões, clique em “Conferir com o banco”.'],
        detalhes: [
          'Em Contas e cartões, clique em “Conferir com o banco”.',
          'Abra o app de cada banco e digite o valor que aparece lá: o saldo de cada conta, a fatura do mês de cada cartão e o valor de cada investimento e reserva.',
          'O site compara na hora: “bate”, ou quanto o app (ou o site) mostra a mais.',
          'Quando não bate, aparecem as pistas: quando o banco atualizou, reservas da conta, lançamentos pendentes, parcelas, estornos e compras repetidas. A pista com o mesmo valor da diferença vem primeiro, marcada “Explica a diferença”.',
          'Cada pista tem o link “ver” para a tela com o detalhe. Nada é gravado: os valores digitados ficam só na tela.',
          'Faça isso depois de sincronizar: o banco atualiza o Open Finance mais ou menos uma vez por dia, e o app mostra o que aconteceu depois disso.'
        ],
        abrir: [{ rotulo: 'Conferir com o banco', rota: '/conferir-banco' }]
      },
      {
        id: 'limite-reduzido',
        titulo: 'Limite reduzido no app do banco',
        paraQueServe: 'Não confundir o limite que você tirou do cartão no app com dívida.',
        topicos: ['Se você reduz o limite no app do banco (por segurança, enquanto não usa), o “limite usado” que o banco…'],
        detalhes: [
          'Se você reduz o limite no app do banco (por segurança, enquanto não usa), o “limite usado” que o banco informa fica bem maior que a dívida do cartão.',
          'Quando isso acontece, o cartão em Contas e cartões pergunta: “Você reduziu o limite no app?”. Clique em “responder” (ou em “editar”).',
          '“Sim, ajustei o limite”: o site usa o limite que ficou no app, e o cartão mostra “limite reduzido por você no app”.',
          '“Não”: a diferença conta como compromisso (compras em processamento ou parcelas). A barra passa a usar o limite aprovado e inclui essa diferença.',
          'Mudou de ideia? Em “editar”, clique em “desfazer”. Depois de um “não”, a pergunta só volta se a diferença mudar bastante.'
        ],
        abrir: [{ rotulo: 'Abrir Contas e cartões', rota: '/contas' }],
        termos: ['limiteUsado']
      },
      {
        id: 'historico-faturas',
        titulo: 'Histórico de faturas',
        paraQueServe: 'Ver as faturas anteriores de um cartão, sem sair da tela.',
        topicos: ['No cartão, clique em “Histórico de faturas”.'],
        detalhes: [
          'No cartão, clique em “Histórico de faturas”.',
          'Aparecem as faturas mais recentes: vencimento, quando fechou, total e situação.',
          'Use “Mostrar mais antigas” para ver o resto.',
          'No Painel, clicar no nome de um cartão em “Faturas do mês” abre direto este histórico.',
          'Cartão manual não tem histórico: as faturas vêm do banco pelo Open Finance.'
        ],
        abrir: [{ rotulo: 'Abrir Contas e cartões', rota: '/contas' }]
      }
    ]
  },
  {
    id: 'transacoes',
    titulo: 'Transações: encontrar e entender',
    resumo: 'Filtros, o que cada etiqueta quer dizer, lançamentos manuais, transferências e edição de uma transação.',
    secoes: [
      {
        id: 'filtros',
        titulo: 'Filtrar e buscar',
        paraQueServe: 'Encontrar rápido as transações que importam.',
        print: { id: 'filtros', alt: 'Transações com os botões do topo, os filtros de período, conta, categoria, tipo e lançamento, e a caixa de busca.' },
        topicos: [
          'Período → “De” e “Até” (o padrão é o mês atual).',
          'Conta, categorias, tipo e “Lançamento” (Pix, compras no cartão…) → filtros acima da lista.',
          'Buscar → digite e aperte Enter (descrição, estabelecimento e observação).',
          'Filtros ativos → ficam em “Filtrando:”; o ✕ tira um, “Limpar filtros” tira todos.'
        ],
        abrir: [{ rotulo: 'Abrir Transações', rota: '/transacoes' }],
        termos: ['tipoLancamento'],
        detalhes: [
          'Marque “Só emergências” ou “Descrição pouco informativa” quando precisar.',
          'Vindo de outra tela com uma lista de lançamentos (o “Ver em Transações” do aviso da sincronização, os trocos do Painel, os FIIs), a origem aparece em “Filtrando:”, por exemplo “Novas e atualizadas na sincronização: 5 lançamentos”. O ✕ tira esse filtro e mostra o período inteiro.',
          'Para ordenar, clique no nome da coluna (Data, Categoria, Conta ou Valor); clique de novo para inverter. Em Valor, o maior lançamento vem primeiro, seja gasto ou receita; as sem categoria ficam no fim. A ordem vale para todas as páginas.',
          'O endereço da página guarda os filtros: dá para salvar nos favoritos ou mandar o link de uma lista filtrada.'
        ]
      },
      {
        id: 'etiquetas',
        titulo: 'O que cada etiqueta quer dizer',
        paraQueServe: 'Entender cada transação sem precisar abri-la.',
        print: { id: 'etiquetas', alt: 'Lista de transações com data, descrição, etiqueta de tipo, categoria, conta e valor.' },
        topicos: [
          'Selo antes do nome → o serviço, o tipo do lançamento ou o ícone da categoria.',
          'Primeira etiqueta → como o dinheiro se moveu: “Compra no cartão”, “Pix enviado”, “Pagamento de fatura”.',
          '“fora dos gastos” → não entra em despesas nem em receitas (transferência, fatura, investimento).',
          '“somando nos gastos” (ou “nas receitas”) → está contando em dobro; veja o capítulo de categorização.'
        ],
        abrir: [{ rotulo: 'Abrir Transações', rota: '/transacoes' }],
        termos: ['tipoLancamento'],
        detalhes: [
          'O selo do serviço aparece quando o site o reconhece pelo texto do banco (Netflix, iFood, Uber…). Senão, vem o selo do tipo (o do Pix; o logo do banco em “Pagamento de fatura”) e, por fim, o ícone da categoria. O mesmo selo aparece no Painel e em Contas a pagar.',
          '“fora dos gastos” também vale para “ignorar nos relatórios”.',
          '“somando nos gastos” aparece num pagamento de fatura ou numa transferência entre contas suas.',
          '“2/10”: parcela 2 de 10. “pendente”: ainda não efetivada. “emergência”: gasto marcado como emergência. “open finance”: veio do banco.',
          'Na coluna Conta, o selo do banco e “cartão de crédito” quando é cartão.'
        ]
      },
      {
        id: 'editar-transacao',
        titulo: 'Abrir e ajustar uma transação',
        paraQueServe: 'Corrigir a categoria, dar um nome melhor ou marcar algo especial numa transação.',
        topicos: [
          'Abrir → clique na linha (no teclado: Tab até o nome e Enter).',
          '“Descrição” → um nome que você reconheça; a do banco continua guardada.',
          '“Categoria” → escolha uma ou use as “Sugestões”; “Automática (regras)” volta às regras.',
          '“Ignorar nos relatórios e totais” e “Gasto de emergência” → marque se fizer sentido e salve.'
        ],
        abrir: [{ rotulo: 'Abrir Transações', rota: '/transacoes' }],
        detalhes: [
          'A descrição original do banco é a que as regras usam.',
          'Use “Observação” para lembrar de algo.',
          'Transações do Open Finance não podem ser excluídas (o banco mandaria de novo). Para tirar dos totais, marque “Ignorar nos relatórios”.'
        ]
      },
      {
        id: 'renomear',
        titulo: 'Renomear: um nome claro, também para as próximas',
        paraQueServe: 'Trocar descrições como “PIX ENVIADO 12345” por um nome que você entenda daqui a meses, e fazer as próximas iguais já chegarem com ele.',
        topicos: [
          'Em Transações, clique no ✎ ao lado da descrição e escreva a “Nova descrição”.',
          '“Onde usar este nome” → só nesta transação, ou também nas outras iguais e nas próximas.',
          'Salve. O aviso tem “Desfazer”; depois, ✎ › “Voltar à descrição do banco”.',
          'Várias de uma vez → marque as transações e use “✎ Renomear” na barra de seleção.'
        ],
        abrir: [
          { rotulo: 'Ver as descrições pouco informativas', rota: '/transacoes', params: { generica: '1' } },
          { rotulo: 'Ver as regras de descrição', rota: '/categorias' }
        ],
        detalhes: [
          'Com “também nas outras”, escolha se a descrição do banco deve “for igual a”, “contiver” ou “começar com” o texto. O site mostra quantas transações mudam antes de salvar. Para PIX e boletos com número que muda todo mês, use “contiver” e deixe só a parte fixa.',
          'Na lista, o nome novo fica em cima e a descrição do banco embaixo, menor (“do banco: …”). A busca acha pelos dois. Uma transação renomeada sai do filtro “Descrição pouco informativa”.',
          'As regras ficam em Categorias › Regras automáticas › “Regras de descrição”. “+ Regra de descrição” cria uma direto, com prévia de quantas transações renomeia; dá para editar, desligar ou excluir.',
          'Uma regra de descrição também pega o nome dado por outra, mas cada regra vale uma vez só para a mesma transação: não há volta sem fim. Quando uma regra muda nomes, as regras de categoria valem de novo nessas transações (menos onde você escolheu a categoria à mão).',
          'Mesma descrição do banco e valores diferentes (duas contas do condomínio)? Preencha “E o valor”, “De” e “Até”, sem sinal (“De R$ 150,00 Até R$ 250,00” pega a despesa de R$ 200,00). Crie uma regra por valor; a com valor vem antes das outras.',
          'Nome trocado à mão (pelo ✎ “Só nesta” ou pelo formulário) não é mudado por nenhuma regra. Para a regra valer nela, use “Também nas outras” de novo a partir dela.'
        ]
      },
      {
        id: 'lancamento-manual',
        titulo: 'Lançamento manual e transferência',
        paraQueServe: 'Registrar o que o banco não informa, como gastos em dinheiro ou uma compra paga direto de um cofrinho.',
        print: { id: 'lancamento-manual', alt: 'Janela Novo lançamento com tipo, conta, data, valor, descrição, categoria, observação e as caixas de marcar.' },
        topicos: [
          'Antes, crie uma conta manual (ex.: “Carteira”) em Contas e cartões.',
          '“+ Lançamento” → tipo, conta, data, valor, descrição e categoria.',
          '“Pendente” → marque para algo que ainda vai acontecer.',
          '“⇄ Transferência” → dinheiro entre contas; não conta como gasto nem como receita.'
        ],
        abrir: [{ rotulo: 'Abrir Transações', rota: '/transacoes' }, { rotulo: 'Criar uma conta manual', rota: '/contas' }],
        detalhes: [
          'Compra paga direto de um cofrinho (no Mercado Livre, no débito de um cofrinho do Mercado Pago)? Em “Conta”, escolha a conta do banco no grupo “Gasto pago por um cofrinho” e o cofrinho no campo que aparece. Entra como despesa e baixa o saldo estimado do cofrinho; o saldo da conta não muda. Na lista, leva a etiqueta “cofrinho …”.',
          'Só aparecem nesse grupo as contas do Open Finance que têm cofrinho em Investimentos › Reservas.',
          'A transferência é lançada só nas contas manuais; o lado do Open Finance chega pela sincronização.'
        ]
      },
      {
        id: 'ignorar-e-emergencia',
        titulo: 'Quando usar “ignorar nos relatórios” e “emergência”',
        paraQueServe: 'Deixar os totais e orçamentos refletindo a sua rotina.',
        topicos: [
          '“Ignorar nos relatórios” → o que não é gasto nem receita (reembolso que entrou e saiu, valor duplicado).',
          '“Gasto de emergência” → despesa fora da rotina (conserto, remédio, viagem urgente).',
          'Em lote → selecione as linhas e use “Marcar emergência” na barra que aparece.'
        ],
        abrir: [{ rotulo: 'Ver só as emergências', rota: '/transacoes', params: { emergencias: '1' } }],
        detalhes: ['A emergência continua nos totais, mas aparece em laranja nos orçamentos e relatórios.']
      },
      {
        id: 'selecionar-varias',
        titulo: 'Selecionar várias transações',
        paraQueServe: 'Aplicar a mesma ação (categoria, nome, emergência…) a várias transações de uma vez.',
        topicos: [
          'Marcar → caixa à esquerda da linha; com Shift, marca o intervalo entre duas.',
          'Caixa do cabeçalho → marca todas desta página.',
          'Barra de ações no topo → diz quantas estão marcadas, inclusive em outras páginas.',
          'Esc ou “✕ Limpar seleção” → desmarca tudo.'
        ],
        abrir: [{ rotulo: 'Abrir Transações', rota: '/transacoes' }],
        detalhes: [
          'Trocar de página mantém a seleção: dá para juntar transações de várias páginas e aplicar a ação de uma vez.',
          'Trocar o filtro (busca, período, categoria…) limpa a seleção, com um aviso. Assim nenhuma ação em lote pega uma transação que não aparece no filtro atual.',
          'Com só algumas marcadas, a caixa do cabeçalho mostra um traço; clique nela para marcar todas.'
        ]
      }
    ]
  },
  {
    id: 'categorizar',
    titulo: 'Categorizar suas transações',
    resumo: 'O que deixa os relatórios certos: corrigir uma, várias de uma vez, sugestões e zerar a fila.',
    secoes: [
      {
        id: 'por-que',
        titulo: 'Por que categorizar faz diferença',
        paraQueServe: 'Quase tudo no site depende da categoria de cada transação.',
        topicos: ['Gastos por categoria no Painel e nos Relatórios.'],
        detalhes: [
          'Gastos por categoria no Painel e nos Relatórios.',
          'Orçamentos e a sugestão de orçamentos.',
          'O assistente de metas (a sobra prevista) e a distribuição da renda (50/30/20).',
          'Categoria errada distorce tudo isso — e “Sem categoria” some das análises por categoria.',
          'Uma rotina de 10 minutos por semana revisando “Sem categoria” e “Descrição pouco informativa” mantém tudo em dia.'
        ]
      },
      {
        id: 'automatica-x-manual',
        titulo: 'Categoria automática × escolhida à mão',
        paraQueServe: 'Saber o que muda sozinho e o que fica como você deixou.',
        topicos: ['Ao chegar, cada transação recebe uma categoria automática: primeiro pelas suas regras, depois pela categoria…'],
        detalhes: [
          'Ao chegar, cada transação recebe uma categoria automática: primeiro pelas suas regras, depois pela categoria que o banco informa.',
          'Quando você escolhe a categoria, ela passa a ser “escolhida à mão” e não muda mais sozinha.',
          'Para devolver à categorização automática, escolha “Automática (regras)”.'
        ]
      },
      {
        id: 'modo-edicao',
        titulo: 'Várias de uma vez: “Editar categorias”',
        paraQueServe: 'Arrumar dezenas de transações rápido, sem abrir uma por uma.',
        print: { id: 'modo-edicao', alt: 'Lista de transações no modo de edição, com uma lista suspensa de categoria em cada linha.' },
        topicos: ['Em Transações, filtre o que quer arrumar (ex.: o mês, “Sem categoria”).'],
        detalhes: [
          'Em Transações, filtre o que quer arrumar (ex.: o mês, “Sem categoria”).',
          'Clique em “✎ Editar categorias”. Cada linha ganha um seletor de categoria.',
          'Troque a categoria nas linhas. A linha fica marcada “alterada” — nada é gravado ainda. Esc numa linha desfaz a troca dela.',
          'Para várias linhas iguais: marque-as e use “Trocar nas selecionadas” na barra de seleção.',
          'Para tudo o que está no filtro (inclusive outras páginas): use “Todas as N transações do filtro” e confirme.',
          'Clique em “Salvar alterações (N)”. O aviso que aparece tem “Desfazer”.',
          'Se várias transações iguais foram para a mesma categoria, a tela oferece “Criar regra” para as próximas.',
          'Mudar de página ou de filtro com trocas não salvas pede confirmação — salve antes para não perder.'
        ],
        abrir: [{ rotulo: 'Editar as sem categoria', rota: '/transacoes', params: { categoria: 'sem', editar: '1' } }]
      },
      {
        id: 'sugestoes',
        titulo: 'Sugestões de categoria',
        paraQueServe: 'Deixar o site propor a categoria pelo seu histórico, em grupos de transações parecidas.',
        topicos: ['Em Transações, clique em “✦ Sugestões de categoria”.'],
        detalhes: [
          'Em Transações, clique em “✦ Sugestões de categoria”.',
          'Escolha “Sem categoria” ou “Revisar automáticas”.',
          'Cada grupo mostra quantas transações tem, a sugestão e a confiança (alta, média, palavra-chave).',
          'Ajuste a categoria se precisar, marque a caixa da coluna “Regra” se quiser que as próximas venham sozinhas, e clique em “Aplicar”.',
          '“Aplicar as N de confiança alta” resolve de uma vez os grupos mais certos.',
          'O texto da regra sai da descrição do banco, que é o que a regra confere. A caixa vem desmarcada, com o motivo embaixo, quando a regra já existe, quando o texto também aparece em transações de outras categorias, ou quando outra linha pede a mesma regra para outra categoria. Sem um texto que identifique só aquele grupo (ex.: Pix para pessoas diferentes), não há regra: categorize sem ela.',
          'Duas linhas com a mesma regra (ex.: “contém MERCADOLIVRE”) e categorias diferentes: só uma regra pode valer. “Aplicar as N de confiança alta” categoriza as duas, mas não cria essa regra; escolha a categoria certa e crie a regra numa das linhas, ou em Categorias e regras.'
        ],
        abrir: [{ rotulo: 'Abrir Transações', rota: '/transacoes' }]
      },
      {
        id: 'zerar-a-fila',
        titulo: 'Zerar a fila: sem categoria e descrição pouco informativa',
        paraQueServe: 'Encontrar o que ainda não tem categoria certa.',
        topicos: ['Abra Transações com o filtro “Sem categoria” e categorize (de preferência no “Editar categorias”).'],
        detalhes: [
          'Abra Transações com o filtro “Sem categoria” e categorize (de preferência no “Editar categorias”).',
          'Marque o filtro “Descrição pouco informativa”: aparecem as transações cuja descrição não diz o que foi (“PIX ENVIADO”, “PAGAMENTO”).',
          'Para cada uma: escolha a categoria, dê um nome melhor na descrição ou clique em “✓ marcar como revisada”.',
          'As notificações (o sino da barra de cima) mostram, discretamente, quantas descrições pouco informativas há nos últimos 90 dias, com o link “Revisar”.'
        ],
        abrir: [
          { rotulo: 'Ver as sem categoria', rota: '/transacoes', params: { categoria: 'sem' } },
          { rotulo: 'Ver as de descrição pouco informativa', rota: '/transacoes', params: { generica: '1' } }
        ],
        termos: ['descricaoPoucoInformativa']
      },
      {
        id: 'gasto-em-dobro',
        titulo: 'Pagamento de fatura e transferências contando em dobro',
        paraQueServe: 'Evitar que o pagamento da fatura (ou dinheiro passando entre suas contas) apareça como gasto novo.',
        topicos: ['As compras do cartão já entram como gasto. Pagar a fatura não é um gasto novo — só tira dinheiro da conta…'],
        detalhes: [
          'As compras do cartão já entram como gasto. Pagar a fatura não é um gasto novo — só tira dinheiro da conta para o cartão.',
          'Filtre “Lançamento” por “Pagamento de fatura” (ou “Entre suas contas”) e procure a etiqueta “somando nos gastos”.',
          'Entre no “Editar categorias”, use “Todas as N transações do filtro” e escolha a categoria “Pagamento de fatura” (ou “Transferências”).',
          'Essas categorias são do tipo transferência: as transações passam a ficar “fora dos gastos”.',
          'Na categoria “Transferências”, só fica fora o que é entre contas suas: o par aparece na outra conta (mesmo valor, até 5 dias), o banco informa o mesmo titular, a transferência foi lançada pelo site ou é dinheiro guardado numa reserva. Pix e TED para outra pessoa (ou recebidos de outra pessoa, como salário) contam como despesa ou receita.',
          'Mandou dinheiro para uma conta sua que não está conectada? O site não vê o par: crie uma categoria do tipo transferência (ex.: “Entre minhas contas”) e use nela. Outras categorias do tipo transferência ficam sempre fora.',
          'Seu banco não informa se o Pix foi para você mesmo? Em Configurações › Open Finance, preencha “Seu nome como aparece nos Pix”: Pix e TED que citam o seu primeiro e último nome passam a contar como entre contas suas.',
          'Pagamento de fatura que o banco mandou como boleto ou “outros” fica fora dos gastos sozinho quando o crédito do mesmo valor aparece num cartão seu em até 5 dias. Sem esse crédito, ele fica como está: “fatura” também pode ser conta de telefone ou de luz.'
        ],
        abrir: [{ rotulo: 'Ver os pagamentos de fatura', rota: '/transacoes', params: { lancamento: 'fatura' } },
          { rotulo: 'Abrir Configurações › Open Finance', rota: '/configuracoes' }],
        termos: ['tipoLancamento']
      },
      {
        id: 'estornos',
        titulo: 'Estornos no cartão',
        paraQueServe: 'Entender por que um estorno aparece em verde, mas não aumenta a sua receita.',
        topicos: ['No cartão, todo crédito que não é o pagamento da fatura é estorno: compra devolvida, “crédito de confiança”…'],
        detalhes: [
          'No cartão, todo crédito que não é o pagamento da fatura é estorno: compra devolvida, “crédito de confiança”, cashback.',
          'O estorno abate o gasto da categoria no mês (ex.: compra de R$ 100 em Compras + estorno de R$ 100 = R$ 0 em Compras) e não conta como receita.',
          'O crédito do pagamento da fatura no cartão (“Pagamento recebido”) fica fora dos gastos, mesmo quando chega sem a categoria “Pagamento de fatura”.',
          'Estorno que o banco manda como transferência (ex.: “Estorno de Uber - NuPay”) ganha a categoria do texto (Uber → Transporte) ou fica sem categoria para você escolher.',
          'Os estornos que já estavam no site foram acertados sozinhos.',
          'Compra parcelada devolvida inteira (o estorno do valor total, ex.: 12 × R$ 460 = R$ 5.527) é uma compra cancelada: o estorno e as parcelas dela já lançadas ficam fora dos relatórios, e no total ela não custa nada. Sem isso, o estorno do valor total abatia o gasto de um mês em que só a 1ª parcela tinha sido cobrada, e as despesas do mês ficavam negativas. Estorno de parte da compra continua abatendo o gasto do mês.'
        ],
        abrir: [{ rotulo: 'Ver os estornos', rota: '/transacoes', params: { lancamento: 'cartao' } }],
        termos: ['tipoLancamento']
      }
    ]
  },
  {
    id: 'regras',
    titulo: 'Regras automáticas',
    resumo: 'Criar regras, prioridade, reaplicar, boas práticas e as categorias que vêm do banco.',
    secoes: [
      {
        id: 'o-que-e-regra',
        titulo: 'O que é uma regra',
        paraQueServe: 'Dizer uma vez “descrição contém UBER → Transporte” e nunca mais categorizar isso à mão.',
        topicos: [
          'A regra compara o texto com a descrição original do banco.',
          'Se bater, a transação recebe a categoria da regra.'
        ],
        detalhes: [
          'As regras valem para as transações que chegam e para as que voltam para “Automática”.'
        ]
      },
      {
        id: 'criar-regra',
        titulo: 'Criar uma regra',
        paraQueServe: 'Categorizar sozinho o que se repete.',
        print: { id: 'criar-regra', alt: 'Aba Regras automáticas de Categorias e regras, com o botão Regra, a tabela de regras e a seção de regras de descrição.' },
        topicos: [
          'Abrir → Categorias e regras › “Regras automáticas” › “+ Regra”.',
          'Preencher → “Quando a descrição” (contém, começa com ou é igual a), “Texto” e “Categoria”.',
          'Conflito → em “Prioridade”, o número menor vence.'
        ],
        abrir: [{ rotulo: 'Abrir Categorias e regras', rota: '/categorias' }],
        detalhes: [
          'Acento e maiúscula não fazem diferença no “Texto”.',
          'O texto vale também pelo nome atual da transação, o dado por você ou por uma regra de descrição.',
          'Também dá para criar regra em “Editar categorias” e nas “Sugestões de categoria”.'
        ]
      },
      {
        id: 'reaplicar',
        titulo: 'Reaplicar as regras',
        paraQueServe: 'Atualizar as transações antigas depois de criar ou mudar regras.',
        topicos: ['Categorias e regras › “Regras automáticas” › “Reaplicar regras em todas as transações”.'],
        abrir: [{ rotulo: 'Abrir Categorias e regras', rota: '/categorias' }],
        detalhes: [
          'Só mudam as transações com categoria automática. As escolhidas à mão ficam como estão.'
        ]
      },
      {
        id: 'boas-praticas',
        titulo: 'Boas práticas',
        paraQueServe: 'Regras que acertam e não atrapalham.',
        topicos: [
          'Texto curto e específico → o nome da loja ou do serviço (“IFOOD”, “NETFLIX”).',
          'Evite “PIX”, “PAGAMENTO” e “COMPRA” → pegam transações demais.'
        ],
        abrir: [{ rotulo: 'Buscar em Transações', rota: '/transacoes' }],
        detalhes: [
          'Antes de criar, procure o texto na busca de Transações para ver o que ele pegaria.',
          'Duas regras brigando? Ajuste a prioridade ou deixe o texto mais específico.'
        ]
      },
      {
        id: 'categorias-do-banco',
        titulo: 'Categorias que vêm do banco',
        paraQueServe: 'Aproveitar a categoria que o banco já informa, ligada às suas categorias.',
        topicos: ['Categorias e regras › “Categorias do banco”: escolha a sua categoria para cada uma do banco.'],
        abrir: [{ rotulo: 'Abrir Categorias e regras', rota: '/categorias' }],
        detalhes: [
          'Esse mapeamento vale quando nenhuma regra sua bate.'
        ]
      }
    ]
  },
  {
    id: 'categorias',
    titulo: 'Categorias',
    resumo: 'Criar categorias e subcategorias, tipo, natureza do gasto e por que transferência não é gasto.',
    secoes: [
      {
        id: 'criar-categoria',
        titulo: 'Criar e organizar categorias',
        paraQueServe: 'Ter as categorias do seu jeito.',
        print: { id: 'criar-categoria', alt: 'Janela Nova categoria com nome, tipo, subcategoria de, natureza do gasto, cor e a caixa Ativa.' },
        topicos: ['Abra Categorias e regras › “Categorias” e clique em “+ Categoria”.'],
        detalhes: [
          'Abra Categorias e regras › “Categorias” e clique em “+ Categoria”.',
          'Dê um “Nome”, escolha o “Tipo” (despesa, receita ou transferência) e uma cor.',
          'Cada categoria aparece com um ícone na cor dela (talheres em Alimentação, casa em Moradia…) no Painel, nas Análises, em Categorias e regras e em Orçamentos. Categoria criada por você aparece com a primeira letra do nome.',
          'Para uma subcategoria (ex.: Lazer › Cinema), escolha a categoria em “Subcategoria de”.',
          'No filtro de Transações, escolher uma categoria inclui as subcategorias dela.'
        ],
        abrir: [{ rotulo: 'Abrir Categorias e regras', rota: '/categorias' }]
      },
      {
        id: 'tipo-transferencia',
        titulo: 'Por que transferência não conta como gasto',
        paraQueServe: 'Evitar contar o mesmo dinheiro duas vezes.',
        topicos: ['Categorias do tipo transferência (como “Transferências” e “Pagamento de fatura”) ficam fora das despesas e…'],
        detalhes: [
          'Categorias do tipo transferência (como “Transferências” e “Pagamento de fatura”) ficam fora das despesas e receitas.',
          'Dinheiro que só muda de conta não é gasto; o pagamento da fatura não é gasto novo, porque as compras já entraram no cartão.',
          'Na lista de Transações, elas aparecem com a etiqueta “fora dos gastos”.'
        ]
      },
      {
        id: 'natureza',
        titulo: 'Essencial × estilo de vida (50/30/20)',
        paraQueServe: 'Ver quanto da renda vai para o necessário e quanto para o resto.',
        topicos: ['Em cada categoria de despesa, escolha a “Natureza do gasto”: essencial (moradia, mercado, saúde) ou estilo de…'],
        detalhes: [
          'Em cada categoria de despesa, escolha a “Natureza do gasto”: essencial (moradia, mercado, saúde) ou estilo de vida (lazer, restaurantes).',
          'A Visão anual e a sugestão de orçamentos usam isso na divisão 50/30/20: até 50% para essenciais, 30% para estilo de vida e pelo menos 20% para guardar.',
          'Na Visão anual, clique em Essenciais, Estilo de vida ou Guardado (na barra ou na legenda) para ver em Transações o que forma cada parte.'
        ],
        abrir: [{ rotulo: 'Abrir a Visão anual', rota: '/anual' }],
        termos: ['comprometimento']
      }
    ]
  },
  {
    id: 'orcamentos',
    titulo: 'Orçamentos',
    resumo: 'Limite por categoria, copiar do mês anterior, sugestão automática e como acompanhar.',
    secoes: [
      {
        id: 'definir-limites',
        titulo: 'Definir os limites do mês',
        paraQueServe: 'Combinar consigo mesmo quanto gastar em cada categoria.',
        print: { id: 'definir-limites', alt: 'Orçamentos do mês: totais orçado, gasto e disponível e a tabela de categorias com o campo de limite de cada uma.' },
        topicos: ['Abra Orçamentos e escolha o mês no seletor.'],
        detalhes: [
          'Abra Orçamentos e escolha o mês no seletor.',
          'Na coluna “Limite (R$)”, digite o limite de cada categoria. Em branco = sem orçamento.',
          'Subcategorias somam no orçamento da categoria pai.',
          'Para repetir os limites, use “Copiar do mês anterior”.'
        ],
        abrir: [{ rotulo: 'Abrir Orçamentos', rota: '/orcamentos' }]
      },
      {
        id: 'sugerir',
        titulo: '✨ Sugerir orçamentos',
        paraQueServe: 'Partir de valores que cabem na sua renda, calculados pelos seus gastos.',
        topicos: ['Clique em “✨ Sugerir orçamentos”.'],
        detalhes: [
          'Clique em “✨ Sugerir orçamentos”.',
          'Cada categoria recebe um valor pela mediana dos últimos 6 meses (sem emergências, compras parceladas e picos isolados), com o motivo.',
          'A soma cabe na renda média menos as parcelas e a meta de poupança; quando não cabe, a tela diz quanto falta.',
          'Revise e edite os valores; categoria que já tem orçamento vem desmarcada.',
          'Clique em aplicar para gravar os marcados.',
          'A meta de poupança e os níveis de uso do cartão ficam em Configurações › Parametrização › Orçamentos.'
        ],
        abrir: [{ rotulo: 'Abrir Orçamentos', rota: '/orcamentos' }]
      },
      {
        id: 'acompanhar',
        titulo: 'Acompanhar durante o mês',
        paraQueServe: 'Perceber cedo quando uma categoria está passando do limite.',
        topicos: ['A barra de cada categoria mostra quanto do limite já foi.'],
        detalhes: [
          'A barra de cada categoria mostra quanto do limite já foi.',
          'Laranja quer dizer que passou do limite só por causa de gastos de emergência.',
          'O sino da barra do topo junta os orçamentos estourados num card, com o link para as despesas de cada categoria. O menu “Ocultar” minimiza o card ou o oculta até o mês que vem.',
          'Clique no gasto de uma categoria para ver as transações por trás dele.'
        ],
        abrir: [{ rotulo: 'Abrir Orçamentos', rota: '/orcamentos' }, { rotulo: 'Abrir o Painel', rota: '/painel' }]
      }
    ]
  },
  {
    id: 'metas',
    titulo: 'Metas',
    resumo: 'Criar metas, acompanhar o progresso, prioridade, quanto guardar por mês e ideias de metas.',
    secoes: [
      {
        id: 'criar-meta',
        titulo: 'Criar uma meta',
        paraQueServe: 'Dar nome, valor e prazo a um objetivo.',
        print: { id: 'criar-meta', alt: 'Janela Nova meta com nome, valor alvo, data alvo, conta, valor inicial, quanto guardar por mês, ícone e cor.' },
        topicos: ['Abra Metas e clique em “+ Nova meta”.'],
        detalhes: [
          'Abra Metas e clique em “+ Nova meta”.',
          'Informe o nome, o “Valor alvo” e, se tiver, a “Data alvo”.',
          'Em “Valor inicial já guardado”, coloque o que você já tem para essa meta.',
          'Não sabe a data? Deixe “Data alvo” em branco (a meta fica “sem data definida”), ou diga em “Quanto vou guardar por mês” quanto vai guardar: o site calcula quando a meta chega ao alvo, contando o mês atual, e preenche a data alvo. A data continua podendo ser mudada à mão. O valor por mês fica gravado na meta (aparece de novo ao editar) e o plano passa a usá-lo; apague o campo para o plano voltar a decidir. O valor não pode passar do que sobra para as metas (a maior sobra dos próximos 12 meses, já descontado o que as outras metas guardam por mês): acima disso, o campo diz o máximo e “Salvar” fica desabilitado.',
          'Escolha um ícone (casa, carro, viagem, estudo, reserva…) e uma cor, e salve. O ícone aparece no cartão da meta e, translúcido, no centro do anel do percentual; o anel e o miolo dele enchem na cor escolhida, conforme o dinheiro guardado.',
          'Cada meta é uma barra que abre e fecha: clique no nome. Fechada, ela mostra o percentual, quanto já foi guardado e quanto guardar no mês. Aberta, o anel, os valores e a prioridade ficam à esquerda, e o plano do mês à direita (no celular, um embaixo do outro). Todas começam abertas; as que você fechar ficam fechadas neste navegador. “Fechar todas” e “Abrir todas” ficam no topo da lista.',
          'Meta sem data alvo mostra “sem data definida”.',
          'Quando a meta chega ao valor alvo, ela fica “Concluída” e sai do plano do mês. Se depois você aumentar o alvo, ela volta para o plano sozinha. Marcada como concluída antes de chegar ao alvo, o cartão diz “Concluída por você”; para ela voltar, desmarque “Concluída” em “Editar”. Aberta, a meta concluída mostra uma caixa verde “Meta alcançada!” com o quanto foi guardado.',
          'Para mudar ou apagar uma meta, use “Editar” ou “Excluir”, no rodapé dela. “Excluir” pergunta antes, com o nome da meta; “Cancelar” (ou Esc) não apaga nada.'
        ],
        abrir: [{ rotulo: 'Abrir Metas', rota: '/metas' }]
      },
      {
        id: 'acompanhar-meta',
        titulo: 'Acompanhar o progresso',
        paraQueServe: 'Fazer a meta andar sozinha ou registrar cada aporte.',
        topicos: ['Sem conta ligada: registre cada valor guardado em “+ Aporte”.'],
        detalhes: [
          'Sem conta ligada: registre cada valor guardado em “+ Aporte”.',
          'Com uma conta ligada (“Acompanhar saldo de uma conta”): o progresso é o valor inicial mais o saldo da conta.',
          'Na conta do Mercado Pago, dá para escolher um cofrinho (reserva): o progresso usa o saldo dele.',
          '“confira em Reservas” aparece quando o saldo do cofrinho é estimado: confira o saldo real em Investimentos › Reservas.'
        ],
        abrir: [{ rotulo: 'Abrir Metas', rota: '/metas' }, { rotulo: 'Conferir as reservas', rota: '/investimentos', params: { aba: 'reservas' } }]
      },
      {
        id: 'quanto-guardar',
        titulo: 'Prioridade e quanto guardar por mês',
        paraQueServe: 'Saber quanto guardar este mês em cada meta, pelo dinheiro que de fato entra e sai, contando a fatura do cartão.',
        topicos: ['Com pelo menos 3 meses de extrato, o topo de Metas mostra a sobra do mês (receitas − fatura que vence −…'],
        detalhes: [
          'Com pelo menos 3 meses de extrato, o topo de Metas mostra a sobra do mês (receitas − fatura que vence − outras despesas) e, numa caixa em destaque, quanto guardar no total (e o limite, quando houver).',
          'Em “Guardar X% da sobra”, escolha quanto da sobra vai para as metas; o resto fica livre para imprevistos.',
          'Use ↑ e ↓ em cada meta para definir a prioridade.',
          'Cada meta mostra “Este mês: guarde R$ X”, com “Consigo guardar outro valor” na mesma linha, e o “Plano mês a mês” já aberto: menos nos meses de fatura alta, mais quando ela cai.',
          'Em “Consigo guardar outro valor”, diga quanto dá para guardar este mês: o resto do plano é refeito na hora e mostra se ainda fecha no prazo.',
          'Na meta em que você disse “Quanto vou guardar por mês”, o plano guarda esse valor todo mês, antes das outras e pela prioridade, mas nunca mais do que a parte da sobra que vai para as metas: se num mês não der, um aviso amarelo diz quanto dá e o resto fica para os meses seguintes.',
          'Quando o prazo não cabe, a meta mostra a data provável, quanto os próximos meses precisariam e onde mexer (prazo, valor ou gastos).',
          '“Plano mês a mês” mostra os próximos 5 meses e para no mês em que a meta é alcançada; “Mostrar mais” acrescenta 5 de cada vez, até 12, e “Mostrar menos” volta aos 5. “Como calculei” explica a conta.',
          '“Como está indo” abre o gráfico da meta: a linha cheia é o total guardado no fim de cada mês, a tracejada é o plano até o valor alvo, e a pontilhada é o alvo. Cada “+ Aporte” muda o gráfico, e embaixo ficam os últimos aportes, com a data e a hora. A escala acompanha os aportes: todos no mesmo dia, um ponto por aporte, na hora e minuto; todos no mesmo mês, um ponto por dia; em meses diferentes, mês a mês, com o plano. O histórico começa no primeiro aporte registrado a partir da versão 2.31.0.',
          'Dinheiro que vai para cofrinho ou investimento precisa estar como transferência; lançado como despesa, ele diminui a sobra.',
          'O valor de “Consigo guardar” é só uma simulação: para valer, registre o aporte na meta.'
        ],
        abrir: [{ rotulo: 'Abrir Metas', rota: '/metas' }, { rotulo: 'Ver a Projeção', rota: '/projecao' }],
        termos: ['sobraDoCaixa', 'parteDaSobra', 'recomendacaoMetas']
      },
      {
        id: 'ideias-de-metas',
        titulo: 'Ideias de metas (e como definir valor e prazo)',
        paraQueServe: 'Começar por objetivos que costumam fazer diferença.',
        topicos: ['Reserva de emergência: de 3 a 6 meses dos seus gastos essenciais (veja a média na Visão anual). Sem data…'],
        detalhes: [
          'Reserva de emergência: de 3 a 6 meses dos seus gastos essenciais (veja a média na Visão anual). Sem data, prioridade 1.',
          'Quitar o cartão ou um empréstimo: o valor da dívida total; prazo curto, porque juros de cartão crescem rápido.',
          'Viagem: o custo estimado (passagem, hospedagem, dia a dia) e a data da viagem.',
          'Entrada de imóvel: em geral 20% do valor do imóvel; prazo de alguns anos.',
          'Troca de carro ou estudos: o valor e a data em que vai precisar.'
        ],
        abrir: [{ rotulo: 'Criar uma meta', rota: '/metas' }],
        termos: ['reservaEmergencia']
      }
    ]
  },
  {
    id: 'contas-a-pagar',
    titulo: 'Contas a pagar',
    resumo: 'Cadastrar vencimentos, pagar, data limite, assinaturas no cartão, débitos automáticos, contas pagas com um cofrinho e as sugestões que vêm do extrato.',
    secoes: [
      {
        id: 'cadastrar-conta',
        titulo: 'Cadastrar uma conta a pagar',
        paraQueServe: 'Ser lembrado dos vencimentos no Painel.',
        print: { id: 'cadastrar-conta', alt: 'Janela Nova conta a pagar com descrição, valor, vencimento, recorrência, categoria, como é cobrada e conta de pagamento.' },
        topicos: [
          'Nova conta → botão “+ Nova conta”, em Contas a pagar.',
          'Descrição, valor, “Próximo vencimento” e recorrência → mensal, anual ou única.',
          '“Como é cobrada” → à mão, débito automático ou no cartão.'
        ],
        abrir: [{ rotulo: 'Abrir Contas a pagar', rota: '/contas-a-pagar' }],
        detalhes: [
          'Conta que acaba (escola até dezembro): preencha “Até (opcional)” ou “quantos pagamentos faltam”.',
          'Escolha a categoria e a conta de onde sai o pagamento.',
          'Use “Observação” para lembretes, como “cancelar em março”.'
        ]
      },
      {
        id: 'cobrada-sozinha',
        titulo: 'Assinaturas e débitos automáticos (cobrados sozinhos)',
        paraQueServe: 'Dar baixa sozinho na conta quando a cobrança chega no extrato.',
        topicos: [
          'Débito automático ou no cartão → escolha em “Como é cobrada”.',
          'Texto da cobrança → preencha “Texto da cobrança no extrato” (ex.: NETFLIX).',
          'Com o texto, a conta não tem “Pagar”: o vencimento avança quando a cobrança chega.'
        ],
        abrir: [{ rotulo: 'Abrir Contas a pagar', rota: '/contas-a-pagar' }],
        detalhes: [
          'O que cai no cartão não aparece nos “Próximos vencimentos” do Painel: já está na fatura.',
          'O texto vale pela descrição original do banco. Renomear a transação não atrapalha.',
          'Sem o texto, a conta continua com o “Pagar”.'
        ]
      },
      {
        id: 'pago-com-cofrinho',
        titulo: 'Conta paga com um cofrinho',
        paraQueServe: 'Registrar a conta que o banco paga direto do cofrinho. Esse pagamento não chega pelo Open Finance.',
        topicos: [
          'Cofrinho → escolha a conta do Open Finance onde ele fica; aparece “Pago com o cofrinho”.',
          'Nome não aparece na lista → use “Outro…” e digite como está no app do banco.',
          'Débito automático → o site lança a despesa no vencimento e baixa o saldo estimado do cofrinho.'
        ],
        abrir: [{ rotulo: 'Abrir Contas a pagar', rota: '/contas-a-pagar' }],
        detalhes: [
          'Com “paga à mão”, o “Pagar” já vem com o cofrinho.',
          'O lançamento sozinho vale desde o dia em que você marca o cofrinho. Vencimento mais antigo fica vencido, com “Pagar”.',
          'Valor diferente (luz, água)? Use “ajustar valor”. Não foi paga assim? Use “desfazer”; o site não lança de novo aquele vencimento.',
          'Se a cobrança chegar pelo extrato mesmo assim, vale a do banco.',
          'Débito automático que não apareceu: o aviso “não veio” pergunta “Foi paga com um cofrinho?” e abre o “Pagar” com o cofrinho.',
          'Pagamento sem despesa: a conta mostra “lançar pelo cofrinho”.',
          'Cofrinho que mudou de nome no banco: a conta avisa “não encontrado”. Edite e escolha o novo.'
        ]
      },
      {
        id: 'assinaturas-recorrentes',
        titulo: 'Assinaturas e recorrentes: onde cada cobrança cai',
        paraQueServe: 'Ver qual assinatura cai em qual cartão e quanto vai por mês.',
        print: { id: 'assinaturas-recorrentes', alt: 'Aba Assinaturas e recorrentes de Contas a pagar, com os totais por mês e por ano e as contas em que cada cobrança cai.' },
        topicos: [
          'Aba “Assinaturas e recorrentes” → em Contas a pagar.',
          'Totais por mês e por ano → no topo; embaixo, um bloco por cartão ou conta.',
          'Achar uma assinatura → digite o nome em “Onde cai…?”.'
        ],
        abrir: [{ rotulo: 'Abrir Assinaturas e recorrentes', rota: '/contas-a-pagar', params: { aba: 'recorrentes' } }],
        detalhes: [
          'A cobrança anual entra no total mensal dividida por 12.',
          'Clique no nome do cartão para fechar ou abrir o bloco. “Fechar todos” e “Abrir todos” fazem de uma vez; o site lembra neste navegador.',
          'Filtre por conta ou cartão e por forma de cobrança. “Mostrar as encerradas” traz as que acabaram.',
          '“ver as cobranças” abre Transações com o último ano daquela conta.',
          'Em Contas e cartões, a linha “N assinaturas · R$ X/mês” abre esta aba já filtrada.',
          'Trocou de cartão? Filtre pelo antigo para ver o que ainda precisa trocar nos serviços.'
        ]
      },
      {
        id: 'avisos-recorrentes',
        titulo: 'Avisos das recorrentes',
        paraQueServe: 'Saber quando uma cobrança que se repete não veio, mudou de cartão ou de valor.',
        topicos: [
          'Avisos → no topo de Contas a pagar, com a ação ao lado.',
          '“Não veio” e “Cartão arquivado” → também chegam no sino da barra de cima.'
        ],
        abrir: [{ rotulo: 'Abrir Contas a pagar', rota: '/contas-a-pagar' }],
        detalhes: [
          '“Não veio”: passou 5 dias do vencimento e a cobrança não está no extrato. Pode ter sido cancelada ou recusada.',
          '“Mudou de cartão”: “Atualizar o catálogo” passa a conta para o novo cartão.',
          '“Aumentou” ou baixou: a última cobrança passou 5% do valor cadastrado. “Atualizar o valor” grava o novo. Conta de valor variável, como a luz, não avisa.',
          '“Cartão arquivado”: a cobrança ainda cai num cartão arquivado. Troque nos serviços.',
          'Ao arquivar um cartão em Contas e cartões, a tela lista o que ainda cobra nele.'
        ]
      },
      {
        id: 'pagar',
        titulo: 'Registrar o pagamento',
        paraQueServe: 'Marcar como paga e ir para o próximo vencimento.',
        topicos: [
          'Pagar → botão “Pagar” na conta; confira a data e confirme.',
          'O vencimento avança um mês (ou um ano); no último pagamento, a conta se encerra.'
        ],
        abrir: [{ rotulo: 'Abrir Contas a pagar', rota: '/contas-a-pagar' }],
        detalhes: [
          'Se a conta de pagamento for manual, ou paga com cofrinho, dá para lançar a despesa junto.'
        ]
      },
      {
        id: 'sugestoes-extrato',
        titulo: 'Sugestões do extrato',
        paraQueServe: 'Cadastrar em um clique os pagamentos que se repetem todo mês ou ano.',
        topicos: [
          'Sugestões → “Sugestões do extrato”: boletos, débitos, Pix e assinaturas que se repetem.',
          '“Cadastrar” → abre o formulário já preenchido; “ignorar” tira a sugestão.',
          '“ver pagamentos” → confere no extrato.'
        ],
        abrir: [{ rotulo: 'Abrir Contas a pagar', rota: '/contas-a-pagar' }],
        detalhes: [
          'Cada sugestão diz como parece ser cobrada e já traz o texto da cobrança.',
          'Parcelas e gastos frequentes (mercado, aplicativo de corrida) ficam de fora.',
          'Assinatura anual só aparece com as duas últimas cobranças no histórico. Se não, cadastre à mão.',
          'Pode trocar o nome ao cadastrar: a conta guarda de qual pagamento veio, e a sugestão não volta.',
          'Conta que você cadastrou à mão também tira a sugestão, se valor (até 15%), dia (até 3 dias) e conta batem.',
          'Na dúvida, a sugestão mostra “parece ser …”. Clique em “é a mesma” para ligar as duas.',
          'Clicou por engano? Use “Desfazer” no aviso. Excluir a conta a pagar também traz a sugestão de volta.'
        ]
      },
      {
        id: 'mesmo-nome-valores-diferentes',
        titulo: 'Duas contas com a mesma descrição do banco',
        paraQueServe: 'Separar cobranças que chegam com o mesmo nome e só mudam no valor, como duas contas do condomínio.',
        topicos: [
          'Regra → Categorias e regras › Regras automáticas › “Regras de descrição” › “+ Regra de descrição”.',
          'Uma regra por conta → “Texto”, faixa em “E o valor” e “Mostrar como”.',
          'Contas já cadastradas → troque o “texto da cobrança” pelo nome da regra.'
        ],
        abrir: [
          { rotulo: 'Abrir Categorias e regras', rota: '/categorias' },
          { rotulo: 'Abrir Contas a pagar', rota: '/contas-a-pagar' }
        ],
        detalhes: [
          'Exemplo: texto CONDOMINIO, faixa de R$ 800,00 a R$ 900,00, mostrar como “Condomínio — taxa”. Repita para a outra conta.',
          'As sugestões passam a mostrar as duas separadas.',
          'Com um texto que as duas têm (só CONDOMINIO), a baixa sozinha e o aviso “aumentou” misturam as contas: use o nome inteiro da regra.',
          'Conta que saiu da faixa volta ao nome do banco. Edite a faixa da regra para renomear de novo.'
        ]
      }
    ]
  },
  {
    id: 'relatorios',
    titulo: 'Relatórios e análises: como ler',
    resumo: 'Painel, Visão anual, Relatórios e Projeção — e as perguntas que cada um responde.',
    secoes: [
      {
        id: 'painel',
        titulo: 'Painel',
        paraQueServe: 'Responder “como estou hoje?” e “como está este mês?”.',
        print: { id: 'painel', alt: 'Começo do Painel: a situação de hoje, com patrimônio líquido, saldo em contas, dívida dos cartões e investimentos.' },
        topicos: [
          '“Situação hoje” → patrimônio, saldo em contas, dívida dos cartões e vencimentos; não muda com o mês.',
          '“Mês de …” → receitas, despesas e resultado; troque o mês com ‹ › ou clicando no nome do mês.',
          '“Gastos por categoria” e “Orçamentos” → clique num valor para ver as transações por trás dele.',
          'Sino na barra de cima → orçamentos estourados e avisos, de qualquer tela.'
        ],
        abrir: [{ rotulo: 'Abrir o Painel', rota: '/painel' }],
        termos: ['dividaCartoes', 'patrimonioSemRendaVariavel'],
        detalhes: [
          'O card “Fatura em aberto” soma a fatura que ainda recebe compras dos cartões em que “Fatura no Painel” está ligado, em Contas e cartões; “ver detalhes” mostra a de cada cartão.',
          'Seletor de mês: o nome do mês abre os 12 meses e o ano, em cima deles, abre uma página de 12 anos (como no calendário dos campos de data). Esc volta um nível.',
          'No sino, “orçamentos estourados” junta as categorias que passaram do limite, as piores primeiro, com quanto passou em reais e em %. Clique no nome da categoria para ver as despesas dela no mês; “Ver todos” mostra as demais. Os avisos trazem o resto (conta vencida, consentimento expirando, falha de sincronização, erros).',
          'O menu “Ocultar” de cada card tem “Minimizar” (o card fica recolhido neste navegador até você clicar em “Mostrar”) e “Ocultar até o mês que vem” ou “até aparecer um novo”. O card volta sozinho se outra categoria estourar ou se uma já estourada piorar de faixa (120%, 150%, 200% ou 300%). O que você oculta sai da conta do sino.',
          '“Mais análises” abre a evolução de 12 meses e as últimas transações. As que têm data depois de hoje (parcelas que o banco já lançou) ficam em “Futuras (N)”. “ver todas” abre Transações no mesmo recorte.',
          '“Gastos por categoria” mostra o total do mês no centro da rosca; as categorias menores que 3% ficam juntas em “Outras”. A lista mostra as 8 maiores; “Mostrar todas” abre o resto. Na fatia “Outras”, o clique abre todas as despesas do mês.',
          '“Orçamentos” mostra cada orçamento do mês com o ícone da categoria e, no rodapé, o total gasto, o total orçado e quantos passaram do limite.'
        ]
      },
      {
        id: 'visao-anual',
        titulo: 'Visão anual',
        paraQueServe: 'Responder “como foi o meu ano?”.',
        print: { id: 'visao-anual', alt: 'Visão anual com o seletor de ano e de comparação e os primeiros indicadores de saúde financeira, como taxa de poupança e reserva de emergência.' },
        topicos: [
          'Poupança acumulada, patrimônio estimado e para onde vai a renda (50/30/20).',
          'Despesas e receitas mês a mês, comparação de categorias com outro ano e mapa de gastos.',
          'Clique nos valores para ver as transações.'
        ],
        abrir: [{ rotulo: 'Abrir a Visão anual', rota: '/anual' }],
        termos: ['comprometimento', 'taxaPoupanca']
      },
      {
        id: 'relatorios-tela',
        titulo: 'Relatórios',
        paraQueServe: 'Responder “para onde foi o meu dinheiro?”.',
        print: { id: 'relatorios-tela', alt: 'Relatórios com os filtros de período e de visão e os totais de receitas, despesas e saldo do período.' },
        topicos: [
          'Fluxo de caixa mensal → barras de receitas e despesas e, por cima, a linha lilás do saldo acumulado.',
          'Despesas (ou receitas) por categoria no período.',
          'Maiores despesas → clique numa para abrir Transações no dia e na conta dela.'
        ],
        abrir: [{ rotulo: 'Abrir Relatórios', rota: '/relatorios' }],
        detalhes: ['O valor de cada ponto da linha está na última coluna da tabela, “Saldo acumulado”.']
      },
      {
        id: 'projecao',
        titulo: 'Projeção',
        paraQueServe: 'Responder “quanto vou ter no fim do mês (e nos próximos)?”.',
        print: { id: 'projecao', alt: 'Projeção com a escolha de 3, 6 ou 12 meses, receita e despesa médias, disponível hoje e saldo previsto.' },
        topicos: [
          'Meses seguintes → média das receitas, média das despesas à vista e parcelas já contratadas.',
          'Mês atual → o que já aconteceu, mais a média do que falta.',
          '“Disponível hoje” → saldo das contas menos as faturas dos cartões.',
          'Gráfico → a tendência; tabela → os números mês a mês.'
        ],
        abrir: [{ rotulo: 'Abrir a Projeção', rota: '/projecao' }],
        termos: ['sobraPrevista'],
        detalhes: [
          'As parcelas que vencem nos meses seguintes ficam fora do “Disponível hoje” e entram na despesa do mês em que vencem: cada parcela conta uma vez só.',
          'Quando o saldo previsto troca de sinal, o mês da virada ganha o selo “saldo volta ao positivo” ou “saldo fica negativo”, e a frase do gráfico diz o mês.',
          '“Gasto médio por categoria” mostra a média por mês das despesas à vista e, ao lado, o último mês comparado com ela (vermelho: mais de 10% acima; verde: mais de 10% abaixo). Clique no nome da categoria para ver as transações dela.'
        ]
      },
      {
        id: 'como-ler-graficos',
        titulo: 'Como ler os gráficos',
        paraQueServe: 'Tirar a conclusão certa de cada gráfico.',
        topicos: [
          'Todo gráfico tem um título, uma frase com a conclusão e uma linha “como ler”.',
          'Com mais de uma série, a legenda diz o que é cada cor.',
          'Com “Ocultar valores” ligado, a frase-resumo também esconde os valores.'
        ]
      }
    ]
  },
  {
    id: 'investimentos',
    titulo: 'Investimentos, reservas e crédito',
    resumo: 'Carteira de renda variável, importação, Selic, FIIs, reservas (cofrinhos), exterior e empréstimos.',
    secoes: [
      {
        id: 'carteira',
        titulo: 'Carteira de renda variável',
        paraQueServe: 'Acompanhar ações, FIIs, ETFs e BDRs com preço médio e proventos.',
        print: { id: 'carteira', alt: 'Investimentos: botões do topo e os resumos de total investido, renda variável, carteira contra a Selic, proventos e Selic.' },
        topicos: [
          'Lançar → “+ Lançar compra/venda”: código da B3 (ex.: MXRF11), quantidade, preço e data.',
          'Abas → Resumo, FIIs, Renda variável e Open Finance.',
          'Resumo → preço médio, cotação, valorização, proventos, câmbio e criptomoedas.'
        ],
        abrir: [{ rotulo: 'Abrir Investimentos', rota: '/investimentos' }],
        termos: ['precoMedio', 'dy', 'dyNaCompra', 'yieldOnCost', 'ptax'],
        detalhes: [
          '“Câmbio”: dólar, euro e iene em reais, pela PTAX de compra do Banco Central. “Mais 7 moedas” traz as outras. Sem internet, fica a última cotação, com a data.',
          '“Mais análises”, no fim do Resumo: distribuição por tipo e rentabilidade de cada ativo contra a Selic.',
          '“Criptomoedas”: Bitcoin, Ethereum, Solana e Tether em reais, pelo último negócio na Binance, atualizado a cada 30 minutos. Com mais de 2 horas, diz “desatualizada”. É preço de referência e muda muito: confira na sua corretora.',
          'Câmbio e criptomoedas não somem com “ocultar valores”, porque não são dados seus.'
        ]
      },
      {
        id: 'importar',
        titulo: 'Importar compras e vendas',
        paraQueServe: 'Não digitar o que já está no extrato ou no Open Finance.',
        topicos: [
          'Do extrato → “⇣ Importar do extrato” acha compras e vendas nas descrições; você escolhe o que importar.',
          'Do banco → aba Open Finance, “Registrar compras na carteira”.'
        ],
        abrir: [{ rotulo: 'Abrir Investimentos', rota: '/investimentos' }]
      },
      {
        id: 'conferir',
        titulo: 'Conferir investimentos',
        paraQueServe: 'Achar e corrigir de uma vez o que costuma sair errado com ações e FIIs do Open Finance.',
        topicos: [
          'Conferir → botão “Conferir investimentos”: compara a carteira com a corretora e o extrato.',
          'Corrigir → desmarque o que não quiser e clique em “Corrigir selecionados”.',
          'Errou? → o aviso tem “Desfazer” por 10 minutos.'
        ],
        abrir: [{ rotulo: 'Abrir Investimentos', rota: '/investimentos' }],
        termos: ['trocoDeCompra', 'taxaB3'],
        detalhes: [
          '“FII cadastrado como outro tipo”: o fundo está como ação e fica fora do total de FIIs.',
          '“Cotas que faltam”: a corretora tem mais cotas que a carteira. Se um débito do extrato explica, ele já vem escolhido, com o preço calculado.',
          '“Troco ou venda contando como rendimento”: vai para “Investimentos”. “Rendimento contando como investimento”: volta a contar como receita.',
          '“Taxa da B3 que falta nas compras”: entra no custo e no preço médio. Só aparece 5 dias depois da compra.',
          'Quando a carteira difere da corretora, a tela de Investimentos avisa no topo.',
          'O troco “sem dúvida” vai sozinho para “Investimentos” depois da sincronização, e o Painel avisa com “Desfazer”. Os outros casos o Painel leva à conferência.',
          'Trocar todos os “Valor recebido de Investimentos” de uma vez tira os proventos da receita. A conferência separa troco e rendimento.'
        ]
      },
      {
        id: 'selic-fiis',
        titulo: 'Comparação com a Selic e FIIs',
        paraQueServe: 'Saber se a carteira rende mais que a renda fixa.',
        topicos: [
          'Selic → o Resumo compara a renda variável com o mesmo valor na Selic, no mesmo período.',
          'FIIs → aba FIIs: renda, compras × hoje, proventos por mês e rentabilidade × Selic e IFIX.'
        ],
        abrir: [{ rotulo: 'Abrir Investimentos', rota: '/investimentos' }],
        termos: ['selic', 'ifix']
      },
      {
        id: 'total-fiis',
        titulo: 'Total da carteira de FIIs',
        paraQueServe: 'Ver quanto os FIIs valem, quanto você investiu e quanto rendem por mês.',
        topicos: [
          'Valor atual e investido → ao lado do título “Meus FIIs”, na aba FIIs.',
          'Linha “Total” → última linha da tabela: investido, valor atual, valorização e renda/mês.'
        ],
        abrir: [{ rotulo: 'Abrir Meus FIIs', rota: '/investimentos', params: { aba: 'fii' } }],
        termos: ['precoMedio', 'yieldOnCost'],
        detalhes: [
          'Valorização: valor atual menos o investido, em R$ e em %.',
          'Fundo sem cotação: entra no investido e na renda, fica fora do valor atual, e o total avisa.',
          'Com “ocultar valores”, o total mostra só os percentuais.',
          'O card “Patrimônio em FIIs” conta o fundo sem cotação pelo preço médio, por isso pode diferir do total da tabela.'
        ]
      },
      {
        id: 'fonte-das-cotacoes',
        titulo: 'Fonte das cotações',
        paraQueServe: 'Saber de onde vêm as cotações e o que sai do seu computador para buscá-las.',
        topicos: [
          'Fonte → aba Renda variável, “fonte das cotações”, ao lado de “Mostrar encerradas”.',
          'Cotações e proventos vêm do Yahoo Finance; a Selic, do Banco Central. Só o código do ativo sai.'
        ],
        abrir: [{ rotulo: 'Abrir a Renda variável', rota: '/investimentos', params: { aba: 'rv' } }],
        detalhes: [
          'Se o Yahoo Finance não responder, o ativo fica com a última cotação, e a próxima atualização tenta de novo.'
        ]
      },
      {
        id: 'mes-a-mes',
        titulo: 'Mês a mês dos FIIs',
        paraQueServe: 'Acompanhar cada mês dos FIIs: quanto entrou, quanto rendeu e quanto as cotas valorizaram.',
        print: { id: 'mes-a-mes', alt: 'Gráfico Mês a mês dos FIIs, com os rendimentos e a valorização de cada mês, e o começo da tabela com os aportes.' },
        topicos: [
          'Mês a mês → aba FIIs; escolha “12 meses” ou “24 meses”.',
          'Gráfico → rendimentos e valorização; tabela → aportes, com o mês mais recente primeiro.',
          '“ver lançamentos” → abre Transações só com o que forma aquele número.'
        ],
        abrir: [{ rotulo: 'Abrir o mês a mês dos FIIs', rota: '/investimentos', params: { aba: 'fii' } }],
        termos: ['valorizacaoMes', 'trocoDeCompra'],
        detalhes: [
          '“Rendimentos” são os proventos que caíram na conta. Troco de compra e valor de venda ficam de fora.',
          'No mês atual, “a receber (data-com)” é o provento que costuma cair no mês seguinte.',
          '“por fundo” abre quanto cada FII pagou. O crédito sem o código do fundo é ligado a ele pelo valor; o que não liga vira “sem fundo identificado”.',
          '“Valorização” é o ganho de preço das cotas no mês, sem aportes nem rendimentos. “Saldo no fim” é o valor da carteira no último dia.',
          'Clicar num mês do gráfico abre os rendimentos dele em Transações. O ✕ tira o filtro.',
          'Aportes são cotas × preço. A taxa da B3 fica no custo: “Conferir investimentos” a calcula.',
          'Compras lançadas à mão entram nos aportes, mas não em “ver lançamentos”.',
          'A valorização pode diferir um pouco do app da corretora: o site usa outra fonte de cotação.'
        ]
      },
      {
        id: 'caixinhas',
        titulo: 'Caixinhas, CDBs e outros investimentos do banco',
        paraQueServe: 'Achar a caixinha do Nubank, o CDB do Mercado Pago e o que mais o banco informa.',
        topicos: [
          'Lista → aba Open Finance: saldo, rentabilidade e comparação com a Selic.',
          'Nome → “renomear” dá o seu nome; a sincronização não muda.',
          '“Rendimento por mês” → embaixo da tabela, por investimento.'
        ],
        abrir: [{ rotulo: 'Abrir Investimentos › Open Finance', rota: '/investimentos', params: { aba: 'of' } }],
        detalhes: [
          'O nome mostra produto e banco, como “CDB · Nubank”. Embaixo fica o nome do banco (“no banco: …”), para achar no app.',
          'Para voltar ao nome automático, abra “renomear” e clique em “Voltar ao nome automático”.',
          '“Rendimento por mês”: quanto o saldo cresceu além do que você guardou ou tirou. “por investimento” abre os outros meses.',
          'A tabela recolhe pelo título “Investimentos do Open Finance”; a escolha fica neste navegador.',
          'O rendimento da caixinha fica no saldo e não vira lançamento no extrato.',
          'O Open Finance manda só o saldo de hoje. O site guarda um por dia, então os meses antigos não aparecem.'
        ]
      },
      {
        id: 'reservas',
        titulo: 'Reservas (cofrinhos do Mercado Pago)',
        paraQueServe: 'Contar no patrimônio o dinheiro guardado em cofrinhos da conta.',
        print: { id: 'reservas', alt: 'Aba Reservas de Investimentos: tabela com cada reserva, a conta, o guardado, o retirado, o saldo e a base do saldo.' },
        topicos: [
          'Saldo → aba Reservas: “informado pelo banco” ou estimado pelo extrato.',
          'Estimado → “conferir saldo” e informe o valor real de vez em quando.',
          'Gasto direto do cofrinho → “registrar saída” na linha, ou “+ Lançamento” em Transações.'
        ],
        abrir: [{ rotulo: 'Abrir as reservas', rota: '/investimentos', params: { aba: 'reservas' } }],
        termos: ['reservaEmergencia'],
        detalhes: [
          'Quando o banco informa só o total da conta, cada cofrinho continua estimado. O que sobra do total aparece em “Reservado” (“restante do total do banco”).',
          'Sem nada do banco, o saldo vem dos lançamentos “Dinheiro reservado” e “Dinheiro retirado”.',
          'Já aparece com outro nome? Confira esta com zero para não contar duas vezes.',
          'Conta paga com cofrinho também baixa a estimativa (veja “Conta paga com um cofrinho”).',
          'Se o saldo informado ficar abaixo da estimativa, marque “Registrar a diferença como gasto do cofrinho”.',
          'Valor muito diferente do esperado pede confirmação.',
          'Reservas “para conferir” geram aviso no sino da barra de cima.'
        ]
      },
      {
        id: 'meli-dolar',
        titulo: 'Méli Dólar (dólar do Mercado Pago)',
        paraQueServe: 'Contar no patrimônio os dólares do Mercado Pago, que o Open Finance não informa.',
        topicos: [
          'Onde → aba Reservas: o Méli Dólar aparece com a quantidade em dólar e a PTAX do dia.',
          'Acertar → “conferir saldo”; em “Informado em”, escolha reais ou dólares.'
        ],
        abrir: [{ rotulo: 'Abrir as reservas', rota: '/investimentos', params: { aba: 'reservas' } }],
        termos: ['ptax'],
        detalhes: [
          'A quantidade vem das compras e vendas do extrato. O saldo em reais usa a última PTAX e entra no patrimônio.',
          'Compra e venda ficam em Investimentos e não contam como gasto nem receita.',
          'A PTAX difere um pouco da cotação do Mercado Pago; a conferência acerta.',
          'Compra com outra categoria posta à mão continua como está. Edite a transação ou use “Reaplicar regras”.'
        ]
      },
      {
        id: 'exterior',
        titulo: 'Ativos e moeda no exterior',
        paraQueServe: 'Somar investimentos em dólar e outras moedas no patrimônio em reais.',
        topicos: [
          'Lançar → escolha a moeda ao registrar a compra (ex.: AAPL); o câmbio vem com a PTAX do dia.',
          'Câmbio → pode trocar pelo da corretora.'
        ],
        abrir: [{ rotulo: 'Abrir Investimentos', rota: '/investimentos' }],
        detalhes: [
          'Os valores são convertidos para reais pela cotação do Banco Central, guardada no computador.'
        ]
      },
      {
        id: 'emprestimos',
        titulo: 'Empréstimos e financiamentos',
        paraQueServe: 'Ver saldo devedor, parcelas e atrasos dos contratos do Open Finance.',
        topicos: [
          'Contratos → Empréstimos e financiamentos; financiamentos e empréstimos aparecem separados.',
          'Cada contrato → saldo devedor, parcelas, taxa, CET e pagamentos.'
        ],
        abrir: [{ rotulo: 'Abrir Empréstimos e financiamentos', rota: '/emprestimos' }],
        termos: ['cet'],
        detalhes: [
          'O patrimônio do Painel já desconta esses saldos devedores.'
        ]
      }
    ]
  },
  {
    id: 'trabalho',
    titulo: 'Trabalho (módulo)',
    resumo: 'O salário pelo bruto, o holerite de cada mês, o 13º, as férias, o FGTS e a rescisão, com o INSS e o IRRF estimados e o histórico. Vem desligado.',
    secoes: [
      {
        id: 'ligar-trabalho',
        titulo: 'Ligar o Trabalho',
        paraQueServe: 'Mostrar no menu o grupo Trabalho, que vem desligado.',
        topicos: [
          'Ligar → Configurações › Módulos, interruptor “Trabalho”.',
          'Menu → o grupo “Trabalho” aparece na hora, com Salário, Holerites, 13º salário, Férias, FGTS e Rescisão.'
        ],
        abrir: [{ rotulo: 'Abrir os módulos', rota: '/configuracoes', params: { aba: 'modulos' } }],
        detalhes: [
          'Com o módulo desligado, o endereço de cada tela mostra o botão “Ligar Trabalho”.',
          'O Trabalho não acessa a internet. Desligar esconde o grupo e guarda os dados.'
        ]
      },
      {
        id: 'salario',
        titulo: 'Salário',
        paraQueServe: 'Ver o bruto, os descontos e o líquido estimado do mês, com o histórico de aumentos.',
        topicos: [
          'Primeira vez → Trabalho › Salário: cadastre o emprego (empregador, admissão, regime, conta, salário bruto).',
          '“O mês” → bruto, INSS, IRRF e líquido; ‹ › troca o mês.',
          'Aumento → “Histórico de salários”: mês, valor novo e “Gravar aumento”.'
        ],
        abrir: [{ rotulo: 'Abrir o Salário', rota: '/trabalho/salario' }],
        detalhes: [
          'É estimativa. Vale o valor do seu holerite.',
          '“Como calculei” mostra a conta linha a linha, com a fonte de cada tabela.',
          'Salário em duas vezes (adiantamento e saldo)? Marque “Recebo em duas vezes”.',
          'O aumento vale para o mês inteiro. “Apagar” tira um aumento; o primeiro salário não sai (grave outro valor no mesmo mês).',
          '“+ Outro emprego” cadastra mais um; “Editar emprego” muda os dados ou exclui.',
          'Estágio e PJ ficam sem INSS e IRRF.',
          'Nada do Trabalho entra nos relatórios: a receita continua sendo a transação do salário na conta.',
          'Há tabelas do INSS e do IRRF de 2025 e 2026. Antes disso, não há estimativa.'
        ]
      },
      {
        id: 'holerites',
        titulo: 'Holerites',
        paraQueServe: 'Montar o holerite de cada mês, ou ler o PDF da empresa, e guardar o histórico.',
        topicos: [
          'Montar → Trabalho › Holerites, mês e “Abrir o holerite do mês”.',
          'Horas extras e descontos → “+ Lançamento”; depois, “Salvar e calcular”.',
          'Ler o PDF → “Ler holerite em PDF”, confira a prévia e “Confirmar e gravar”.',
          'Salário na conta → “Recebido na conta” liga o crédito ao holerite.'
        ],
        abrir: [{ rotulo: 'Abrir os Holerites', rota: '/trabalho/holerites' }],
        detalhes: [
          'O salário vem do Trabalho › Salário. O mês tem 30 dias, como na folha.',
          'Lançamento com “Repetir nos próximos meses” já vem no mês novo.',
          '“Dias úteis” e “Domingos e feriados” vêm do calendário. Acerte os feriados estaduais, os municipais e os de data móvel.',
          '“Fechar o mês” congela os números. “Reabrir” volta a calcular com o salário e as tabelas de agora.',
          'PLR tem imposto próprio: deixe “Tem INSS” e “Tem IRRF” desmarcados.',
          'Hora extra à noite: lance as horas com o percentual somado (50% sobre 20% dá 80%), sem lançar o adicional noturno de novo.',
          'Em “Recebido na conta”, o site sugere os créditos de “Parece o salário”. “Outros créditos do período” deixa ligar qualquer um. “Desligar” desfaz.',
          'O holerite não é receita: a receita continua sendo a transação do salário.',
          'PDF com senha (parte do CPF, data de nascimento): o site pede e não guarda. O PDF é lido neste computador e não fica guardado.',
          'Na prévia, troque o tipo ou o valor de uma linha, ou deixe-a de fora. A prova real avisa se a soma não bate com o total do PDF.',
          'O tipo escolhido para uma linha desconhecida vale nos meses seguintes.',
          'O holerite lido entra fechado, como “Lido do PDF”. O aviso traz “Desfazer” por alguns segundos.',
          '“Este PDF é uma imagem”: só lê PDF com texto. Digite o holerite.',
          '“O PDF passa do limite”: até 700 KB e 5 páginas.',
          '“O holerite do mês está fechado”: reabra o mês antes de gravar o lido.'
        ]
      },
      {
        id: 'decimo-terceiro',
        titulo: '13º salário',
        paraQueServe: 'Saber quanto vem de 13º, em quais datas, e quanto já veio adiantado.',
        topicos: [
          'Abrir → Trabalho › 13º salário; o ano troca em ‹ ›.',
          '1ª parcela (até 30/11) e 2ª (até 20/12) → valores e datas na tela.'
        ],
        abrir: [{ rotulo: 'Abrir o 13º salário', rota: '/trabalho/decimo-terceiro' }],
        detalhes: [
          'O 13º conta cada mês em que você trabalhou 15 dias ou mais.',
          'Veio outra 1ª parcela (com as férias, por exemplo)? Informe em “A 1ª parcela que veio”. “Voltar ao padrão” desfaz.',
          '“Como calculei” mostra cada passo.',
          'Saiu no ano? O 13º proporcional vem na rescisão.',
          'Estágio e PJ não têm 13º.'
        ]
      },
      {
        id: 'ferias',
        titulo: 'Férias',
        paraQueServe: 'Ver os dias de férias, agendar e calcular quanto vem, inclusive vendendo dias.',
        topicos: [
          'Dias → Trabalho › Férias: cada período mostra direito, agendados, vendidos e o prazo.',
          'Agendar → período, início, dias e dias vendidos (até 1/3).',
          '“Ver a conta” → quanto cai na conta; “Comparar” → tirar tudo × vender 1/3.'
        ],
        abrir: [{ rotulo: 'Abrir as Férias', rota: '/trabalho/ferias' }],
        detalhes: [
          '“Vencida”: o prazo passou com dias sobrando, e a empresa paga em dobro. “Vencendo”: faltam 60 dias ou menos.',
          'Faltas sem justificativa diminuem os dias de direito. Digite as faltas do período.',
          'As férias vão em até 3 frações: uma de 14 dias ou mais e as outras de 5 ou mais.',
          'A 1ª parcela do 13º pode vir junto, uma vez por ano, até 30/11. Ela aparece em Trabalho › 13º salário.',
          '“Desmarcar” apaga um agendamento e devolve os dias.',
          'O salário do mês seguinte vem menor: os dias de férias já foram pagos.',
          'As férias que sobram na saída vêm na rescisão.'
        ]
      },
      {
        id: 'fgts',
        titulo: 'FGTS',
        paraQueServe: 'Acompanhar quanto entra de FGTS por mês, o saldo e os saques.',
        topicos: [
          'Saldo → Trabalho › FGTS: estimado mês a mês desde a admissão.',
          'Conferir → “Lançar”, “Saldo que vi no app do FGTS”: a estimativa recomeça dali.',
          'Saques → “Lançar”, com o motivo.'
        ],
        abrir: [{ rotulo: 'Abrir o FGTS', rota: '/trabalho/fgts' }],
        detalhes: [
          'A estimativa não conta a TR. Confira com o app de vez em quando.',
          '“Os últimos meses” mostra a correção, o depósito, os saques e o saldo; “conferido” marca o mês igual ao do app.',
          'Com mais de um emprego CLT, escolha o emprego no alto da tela.',
          'A empresa deposita até o dia 20 do mês seguinte. Diferença grande pode ser depósito que não foi feito.',
          '“Excluir” apaga um lançamento, e o saldo é recalculado.',
          'O FGTS não entra no patrimônio do Painel nem nos relatórios. O saque entra pela transação na conta.',
          'Estágio e PJ não têm FGTS.'
        ]
      },
      {
        id: 'rescisao',
        titulo: 'Rescisão',
        paraQueServe: 'Simular quanto você recebe ao sair do emprego e registrar a rescisão real.',
        topicos: [
          'Simular → Trabalho › Rescisão: tipo, aviso prévio, data de saída e “Simular”.',
          '“Comparar os tipos” → quanto cai na conta e do FGTS em cada tipo.',
          'Registrar → “Registrar esta rescisão” fecha o emprego; “Desfazer o registro” reabre.'
        ],
        abrir: [{ rotulo: 'Abrir a Rescisão', rota: '/trabalho/rescisao' }],
        detalhes: [
          '“A conta” mostra as verbas e os descontos. O FGTS mostra a multa e quanto dá para sacar.',
          'Conferiu o saldo do FGTS no mês da saída? A rescisão usa esse saldo.',
          '“Estou no saque-aniversário do FGTS” vem marcado pelos seus saques. Desmarque se voltou ao saque-rescisão.',
          'Na justa causa, só vêm o saldo de salário e as férias vencidas.',
          'Férias e aviso indenizados não têm INSS nem IR.'
        ]
      }
    ]
  },
  {
    id: 'noticias',
    titulo: 'Notícias (módulo)',
    resumo: 'As notícias do dia do G1, da BBC News Brasil e do TabNews. Vem desligado, porque acessa a internet.',
    secoes: [
      {
        id: 'noticias',
        titulo: 'Notícias',
        paraQueServe: 'Ver as principais notícias do dia no mesmo lugar das finanças.',
        topicos: [
          'Ligar → Configurações › Módulos, interruptor “Notícias” (ou o botão “Ligar Notícias” na tela).',
          'Ler → clique no título; abre o site da notícia em outra aba.',
          'Filtrar → os botões do topo escolhem a fonte; “Atualizar agora” busca na hora.'
        ],
        abrir: [{ rotulo: 'Abrir as Notícias', rota: '/noticias' }, { rotulo: 'Abrir os módulos', rota: '/configuracoes', params: { aba: 'modulos' } }],
        detalhes: [
          'Ligado, o site acessa g1.globo.com, feeds.bbci.co.uk e www.tabnews.com.br. Desligado, nada sai do computador.',
          'Ele busca a cada 45 minutos e guarda as 20 mais novas de cada fonte.',
          '“Atualizar agora” funciona no máximo uma vez a cada 5 minutos.',
          'Só o pedido sai do computador, sem dado seu. As fontes veem o endereço de internet da sua casa, como em qualquer site.',
          'As imagens das notícias não aparecem.',
          '“a última tentativa falhou”: a fonte não respondeu. As outras continuam. Os detalhes ficam em Configurações › Erros.',
          '“Não deu para ler as notícias”: clique em “Tentar de novo”.'
        ]
      }
    ]
  },
  {
    id: 'configuracoes',
    titulo: 'Configurações',
    resumo: 'Parametrização, módulos, aparência, segurança e os erros do sistema.',
    secoes: [
      {
        id: 'parametrizacao',
        titulo: 'Parametrização',
        paraQueServe: 'Ajustar o comportamento do site sem mexer em arquivo nenhum.',
        topicos: ['Abra Configurações › Parametrização.'],
        detalhes: [
          'Abra Configurações › Parametrização.',
          'Cada parâmetro mostra o que faz, a faixa válida e o padrão: sincronização, Painel, segurança do login e por quanto tempo o histórico de acessos fica guardado, orçamentos, metas, transações e erros.',
          'Mude o valor e salve: vale na hora.',
          'Mudar a segurança do login (tentativas antes do bloqueio, tempo de bloqueio e por quanto tempo o histórico de acessos fica guardado) pede a sua senha atual: o campo aparece ao lado de “Salvar parâmetros”. Assim, quem achar o site aberto não afrouxa o bloqueio.',
          'O bloco “Sistema” mostra a versão, os caminhos e o tamanho do banco.',
          '“A senha atual não confere”: a senha digitada não é a de entrar no site. Digite de novo e salve; nada foi gravado.'
        ],
        abrir: [{ rotulo: 'Abrir a Parametrização', rota: '/configuracoes', params: { aba: 'parametrizacao' } }]
      },
      {
        id: 'modulos',
        titulo: 'Módulos',
        paraQueServe: 'Ligar só as partes do site que vão além do básico das finanças e que você quer usar. Desligado, o módulo some do menu, não roda nada em segundo plano e não acessa a internet.',
        print: { id: 'modulos', alt: 'Configurações, aba Módulos: interruptor do módulo Notícias, desligado, com a explicação do que ele acessa.' },
        topicos: ['Abra Configurações › Módulos.'],
        detalhes: [
          'Abra Configurações › Módulos.',
          'Cada módulo mostra o que faz e se acessa a internet (e para onde). Ligue ou desligue pelo interruptor: vale na hora, sem recarregar a página, e em qualquer navegador.',
          'Ligado, a tela do módulo aparece no menu, e o que ele faz em segundo plano (buscar as notícias, por exemplo) roda na hora. Desligado, ela some; se você abrir o endereço dela, o site mostra que o módulo está desligado, com o botão para ligar.',
          'Desligar não apaga nada: os dados do módulo ficam guardados e voltam quando você ligar de novo.',
          'Módulo que acessa a internet vem desligado. Ligar e desligar aparecem nos acessos recentes (Configurações › Segurança).',
          '“Não deu para ler os módulos”: o site não respondeu na hora (por exemplo, reiniciando depois de uma atualização). O aviso aparece no menu e em Configurações › Módulos: clique em “Tentar de novo”, que diz “Tentando…” enquanto lê. Enquanto isso, as telas dos módulos ficam fora do menu.'
        ],
        abrir: [{ rotulo: 'Abrir os módulos', rota: '/configuracoes', params: { aba: 'modulos' } }]
      },
      {
        id: 'aparencia',
        titulo: 'Aparência',
        paraQueServe: 'Deixar o site com as suas cores.',
        print: { id: 'aparencia', alt: 'Configurações, aba Aparência: as paletas de cores Lavanda, Oceano, Floresta, Grafite, Pôr do sol e Sua cor.' },
        topicos: ['Abra Configurações › Aparência.'],
        detalhes: [
          'Abra Configurações › Aparência.',
          'Escolha uma paleta pronta (cada uma tem versão clara e escura) ou uma cor personalizada.',
          'A escolha fica salva neste navegador.'
        ],
        abrir: [{ rotulo: 'Abrir a Aparência', rota: '/configuracoes', params: { aba: 'aparencia' } }]
      },
      {
        id: 'seguranca',
        titulo: 'Segurança',
        paraQueServe: 'Trocar a senha e ver os acessos recentes.',
        print: { id: 'seguranca', alt: 'Configurações, aba Segurança: formulário para alterar a senha e a lista de acessos recentes.' },
        topicos: ['Abra Configurações › Segurança.'],
        detalhes: [
          'Abra Configurações › Segurança.',
          'Em “Alterar senha”, informe a atual e a nova duas vezes. As outras sessões abertas são encerradas.',
          '“Acessos recentes” mostra os últimos acessos e mudanças: quando você entrou ou saiu do site, senha errada, login bloqueado, senha e parâmetros alterados, módulos ligados ou desligados, credenciais da Pluggy, conexões do Open Finance adicionadas ou removidas, as mudanças em várias transações de uma vez (categoria, renomear, regras de descrição) e as conferências de investimentos, com o “Desfazer” de cada uma. “Acesso local” quer dizer que veio deste computador; com o acesso pelo celular ligado, o celular aparece pelo IP dele na rede.',
          'Os acessos ficam guardados por 180 dias e depois são apagados sozinhos, uma vez por dia. Para mudar o prazo (de 30 a 730 dias), use Configurações › Parametrização › Segurança, “Guardar o histórico de acessos por”.'
        ],
        abrir: [{ rotulo: 'Abrir a Segurança', rota: '/configuracoes', params: { aba: 'seguranca' } }]
      },
      {
        id: 'acesso-pelo-celular',
        titulo: 'Acesso pelo celular',
        paraQueServe: 'Abrir o site no celular, na mesma rede Wi-Fi de casa. Vem desligado: assim, o site só abre neste computador.',
        print: { id: 'acesso-pelo-celular', alt: 'Bloco Acesso pelo celular com o aviso de segurança, o endereço do computador na rede e o botão para ligar.' },
        topicos: ['Abra Configurações › Segurança e, em “Acesso pelo celular”, escolha o endereço deste computador na rede (o do…'],
        detalhes: [
          'Abra Configurações › Segurança e, em “Acesso pelo celular”, escolha o endereço deste computador na rede (o do Wi-Fi ou do cabo da sua casa).',
          'Clique em “Ligar o acesso pelo celular” e confirme. Ligado, qualquer aparelho da sua rede chega à tela de entrar: use uma senha forte e não ligue em rede pública (café, aeroporto).',
          'No Windows, a porta abre no firewall só para a rede de casa, e só se a rede estiver como Particular. Se a tela avisar que a rede está como Pública e esta for a sua casa, mude para Particular em Configurações › Rede e Internet. No Linux, se o firewall (ufw ou firewalld) estiver ligado, a tela mostra o comando para rodar uma vez.',
          'No celular, no mesmo Wi-Fi, abra o endereço do certificado que a tela mostra (termina em /api/celular/autoridade): aponte a câmera para o QR code “1. Certificado” ou digite o endereço. Desta vez o navegador avisa que a conexão não é particular: é esperado, siga em “Avançado” e “Continuar”.',
          'Android: abra o arquivo baixado ou vá em Configurações › Segurança › Criptografia e credenciais › Instalar um certificado › Certificado de CA. iPhone: em Ajustes › Perfil Transferido, toque em Instalar; depois, em Ajustes › Geral › Sobre › Ajustes de Confiança de Certificados, ligue “Finanças Local (celular)”.',
          'Para conferir que o certificado é deste computador, compare a impressão digital da tela com a que o iPhone mostra em “Mais detalhes” ao instalar.',
          'Depois disso, abra no celular o endereço que a tela mostra (https://, o IP e :5443), pelo QR code “4. O site” ou digitando, e entre com o seu usuário e a sua senha. Fora de casa, o celular não chega ao site.',
          'Se o roteador der outro IP a este computador, a tela avisa: desligue e ligue de novo com o endereço novo (o certificado do celular continua valendo). Para o endereço não mudar, reserve o IP no roteador.',
          '“Desligar o acesso pelo celular” fecha a porta na hora e apaga a regra do firewall do Windows. Ligar e desligar aparecem nos acessos recentes.',
          'O gerenciador (a janela da bandeja) também liga e desliga, no cartão “Acesso pelo celular”, que mostra se o acesso está ligado e o endereço. “Ligar o acesso” usa o endereço escolhido da última vez (ou o principal deste computador); no Windows, ele pede a permissão de administrador. Para escolher outro endereço, ver o QR code ou os avisos do firewall, use “Ver no site”, que abre esta tela.'
        ],
        abrir: [{ rotulo: 'Abrir a Segurança', rota: '/configuracoes', params: { aba: 'seguranca' } }]
      },
      {
        id: 'erros-do-sistema',
        titulo: 'Erros do sistema',
        paraQueServe: 'Ver o que deu errado (sincronização, cotações, telas) sem procurar em logs.',
        print: { id: 'erros-do-sistema', alt: 'Configurações, aba Erros: filtros de situação, origem, nível e datas e a tabela de erros do sistema.' },
        topicos: ['Abra Configurações › Erros.'],
        detalhes: [
          'Abra Configurações › Erros.',
          'Filtre por situação, origem, nível e período. O mesmo erro repetido aparece uma vez, com quantas vezes aconteceu.',
          '“detalhe” mostra a explicação técnica; “Resolvido” tira da lista (e “Reabrir” traz de volta).',
          'Tokens, senhas, CPF e valores são removidos antes de gravar.',
          '“Reportar para o Claude” copia o erro (a mensagem, a tela, quantas vezes, a explicação técnica e a versão do site, sem o seu login) num texto que começa com /reportar-erro. Cole no Claude Code: ele procura se já existe uma issue, analisa o erro e pergunta se resolve agora, só cataloga ou se é do ambiente de desenvolvimento. O site não manda nada para a internet: quem envia é você, ao colar.',
          'Se o navegador não deixar copiar sozinho, o texto aparece embaixo do erro, já selecionado: copie com Ctrl+C.',
          'O repositório do projeto é público. Antes de abrir uma issue, o Claude relê o texto e tira o que sobrou de dado pessoal (nomes, descrições de transação).'
        ],
        abrir: [{ rotulo: 'Abrir os erros do sistema', rota: '/configuracoes', params: { aba: 'erros' } }]
      }
    ]
  },
  {
    id: 'privacidade',
    titulo: 'Privacidade, backup e atualização',
    resumo: 'O que fica no computador, o que sai, como fazer backup e atualizar.',
    secoes: [
      {
        id: 'o-que-sai',
        titulo: 'O que fica e o que sai do computador',
        paraQueServe: 'Saber exatamente para onde vão os seus dados.',
        topicos: ['Fica no computador: todas as transações, contas, categorias, metas, orçamentos e a carteira.'],
        detalhes: [
          'Fica no computador: todas as transações, contas, categorias, metas, orçamentos e a carteira.',
          'Sai, por HTTPS: a conversa com a Pluggy (Open Finance), com as credenciais que você informou; datas para a taxa Selic e a cotação de moedas no Banco Central; códigos de ativos para as cotações (Yahoo Finance); e o pedido das cotações de criptomoedas à Binance, só com os pares fixos (Bitcoin, Ethereum, Solana e Tether, em reais).',
          'O gerenciador do Windows consulta o GitHub para saber se há versão nova e para baixá-la, sem mandar nada seu. O do Linux só vai ao GitHub na instalação de desenvolvimento, quando você clica em “Atualizar para a última do develop”.',
          'Não sai: valores, quantidades e dados pessoais.',
          'Com o acesso pelo celular ligado (Configurações › Segurança), o site também atende os aparelhos da sua rede de casa, que chegam à tela de entrar. Nada vai para a internet por causa disso, e fora de casa o celular não chega ao site.',
          'Do Pix e da TED, o nome e o CPF de quem pagou ou recebeu não são guardados à parte: fica o meio e se as duas pontas são a mesma pessoa. Mas a descrição vem do banco como está e costuma trazer o nome (“PIX ENVIADO FULANO”); ela fica guardada, só no seu computador.'
        ]
      },
      {
        id: 'ocultar-valores',
        titulo: 'Ocultar valores',
        paraQueServe: 'Mostrar a tela a alguém sem mostrar quanto você tem.',
        topicos: ['Clique no botão do olho na barra de cima.'],
        detalhes: [
          'Clique no botão do olho na barra de cima.',
          'Todos os valores ficam escondidos, inclusive nas frases dos gráficos. O olho fica riscado e o botão destacado enquanto os valores estão escondidos.',
          'Clique de novo para mostrar.'
        ]
      },
      {
        id: 'backup',
        titulo: 'Backup',
        paraQueServe: 'Não perder os seus dados se o computador der problema.',
        topicos: ['Pelo gerenciador (Windows e Linux): abra “Mais opções”, clique em “Salvar uma cópia do banco…” e escolha onde…'],
        detalhes: [
          'Pelo gerenciador (Windows e Linux): abra “Mais opções”, clique em “Salvar uma cópia do banco…” e escolha onde guardar (pendrive, disco externo, nuvem). No Windows, ele pede permissão de administrador.',
          'Instalou pelos scripts? Você também pode rodar scripts/geral/4-backup.sh (de preferência agendado), que copia o banco para a pasta “backups” do projeto. Copie essa pasta de vez em quando para um disco externo ou outro lugar seguro.',
          'A cópia tem todos os seus dados financeiros: guarde em lugar seguro.',
          'Ative a criptografia do disco (BitLocker no Windows, LUKS no Linux): o arquivo do banco não é criptografado.'
        ]
      },
      {
        id: 'restaurar',
        titulo: 'Restaurar uma cópia',
        paraQueServe: 'Voltar o banco para uma cópia salva, depois de um problema.',
        topicos: ['Pelo gerenciador (Windows e Linux): abra “Mais opções” e clique em “Restaurar uma cópia…”. Ele diz o que muda…'],
        detalhes: [
          'Pelo gerenciador (Windows e Linux): abra “Mais opções” e clique em “Restaurar uma cópia…”. Ele diz o que muda e, se você seguir, pede o arquivo da cópia. No Windows, pede permissão de administrador.',
          'O gerenciador confere que o arquivo é uma cópia do banco do Finanças Local antes de desligar o site. Depois desliga, guarda o banco de agora na pasta dados, em “antes-de-restaurar”, põe a cópia no lugar (sem os arquivos -wal e -shm do banco antigo) e liga o site de novo. Se algo der errado no meio, ele devolve o banco de antes.',
          'Abra o site, entre e confira a última transação e Configurações › Erros. Deu tudo certo? Apague a pasta “antes-de-restaurar”.',
          'Sem o gerenciador, à mão: pare o site (“Desligar” no gerenciador, ou o serviço do Finanças Local, se instalou pelos scripts).',
          'Ache a pasta de dados: C:\\ProgramData\\FinancasLocal\\dados, se instalou pelo .msi (só abre como administrador), ou a pasta “dados” do projeto, se instalou pelos scripts.',
          'Guarde o banco atual: copie os arquivos FinancasLocal.db* da pasta dados/sql para outra pasta.',
          'Copie a cópia por cima de dados/sql/FinancasLocal.db.',
          'Apague os arquivos FinancasLocal.db-wal e FinancasLocal.db-shm da pasta dados/sql. Se ficarem, a cópia pode corromper.',
          'Ligue o site (“Ligar” no gerenciador), abra-o e confira a última transação.',
          'Tudo volta para a data da cópia, inclusive a senha do site: entre com a senha que valia naquele dia.',
          'O que veio do Open Finance depois da cópia volta na próxima sincronização; o que você lançou à mão, não.',
          'Antes de cada atualização que muda o banco, o sistema guarda uma cópia em dados/backups.',
          'Os comandos prontos, para Windows e Linux, estão em docs/OPERACAO.md, no repositório do projeto.'
        ]
      },
      {
        id: 'atualizar',
        titulo: 'Atualizar e parar o sistema',
        paraQueServe: 'Manter o site em dia e saber desligar tudo.',
        topicos: ['Pelo gerenciador do Windows (instalação pelo .msi): quando aparecer “Tem versão nova”, veja “O que mudou” e…'],
        detalhes: [
          'Pelo gerenciador do Windows (instalação pelo .msi): quando aparecer “Tem versão nova”, veja “O que mudou” e clique em “Atualizar para …”. Ele baixa, confere e abre o instalador; é só avançar. O site fica fora do ar por alguns segundos. Sem o gerenciador, baixe o .msi novo da página de versões no GitHub e abra.',
          'Nenhuma atualização apaga dados. Antes de mudar a estrutura do banco, a versão nova guarda uma cópia em dados/backups.',
          'Para parar o site: “Desligar” no gerenciador; “Ligar” volta. Sem isso, o site liga sozinho com o computador.',
          'Instalou pelos scripts: para atualizar, rode scripts/geral/instalar.sh na pasta do projeto, que também traz o .NET e o Go das versões novas e faz backup do banco antes; para parar tudo, parar-windows.cmd no Windows ou scripts/geral/parar.sh no Linux.'
        ]
      },
      {
        id: 'problemas-comuns',
        titulo: 'Problemas comuns',
        paraQueServe: 'Resolver sozinho os tropeços mais frequentes.',
        topicos: ['O navegador alerta sobre o certificado: abra o mesmo .msi de novo e escolha “Reparar”, que recoloca o…'],
        detalhes: [
          'O navegador alerta sobre o certificado: abra o mesmo .msi de novo e escolha “Reparar”, que recoloca o certificado nos confiáveis. Instalou pelos scripts? Rode scripts/geral/instalar.sh de novo.',
          '“Não foi possível falar com a API”: abra o gerenciador e veja a situação. Desligado, clique em “Ligar”; “ligado, mas não responde”, espere alguns segundos e clique em “Reiniciar”.',
          'O Client Secret da Pluggy não abre mais (depois de trocar de computador): informe o Client Secret de novo em Configurações.',
          'Algo deu errado e você não sabe o quê: veja Configurações › Erros.'
        ],
        abrir: [{ rotulo: 'Ver os erros do sistema', rota: '/configuracoes', params: { aba: 'erros' } }]
      }
    ]
  }
];
