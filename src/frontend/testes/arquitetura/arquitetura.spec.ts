// Teste de arquitetura do front-end (#476), no estilo do ArchUnit do Java: quem pode importar quem em src/app.
//
// Só local, por decisão de 29/10/2026: fica fora de testes/jest/ (o jest.config.js só procura lá), então o `npm test` e
// a CI não o rodam. Roda com `npm run test:arquitetura`. Os imports são lidos do código com uma expressão regular, como os
// outros testes que leem o código, sem dependência nova. Mudou a arquitetura de propósito? Mude a tabela aqui no mesmo
// PR, com o motivo.
import { readdirSync, readFileSync } from 'fs';
import { dirname, join, normalize, relative, sep } from 'path';

const APP = join(__dirname, '..', '..', 'src', 'app');

/** Arquivo (relativo a src/app, com "/") → o que ele importa (relativo a src/app, sem extensão, ou o pacote). */
type Grafo = Record<string, { imports: string[]; texto: string }>;

/* ------------------------------------------------------------ as tabelas */

/** Regra 1: o HttpClient é injetado só pelo ApiService, o único ponto de chamadas HTTP (CLAUDE.md). */
const PODEM_INJETAR_HTTP: Record<string, string> = {
  'core/api.service.ts': 'o único ponto de chamadas HTTP (CLAUDE.md)',
  'core/auth.service.ts':
    'exceção: login, sessão e sair guardam o estado da autenticação, usado pelos guards antes do resto; passar para o ' +
    'ApiService mexe no fluxo de login'
};

/** Regras 2 a 4: o que cada pasta não importa. */
const PASTAS_PROIBIDAS: Record<string, { proibidas: string[]; motivo: string }> = {
  core: { proibidas: ['compartilhado', 'layout', 'paginas'], motivo: 'o core é a base: serviços e modelos' },
  compartilhado: { proibidas: ['paginas', 'layout'], motivo: 'o componente reutilizável não sabe de uma tela' },
  layout: { proibidas: ['paginas'], motivo: 'a casca da tela não depende de uma página' }
};

/** Regra 6 (#634, ADR 0058): dentro do compartilhado, o que não tem assunto do domínio não depende de um assunto. */
const SUBPASTAS_PROIBIDAS: Record<string, { proibidas: string[]; motivo: string }> = {
  'compartilhado/ui': {
    proibidas: ['compartilhado/bancos', 'compartilhado/lancamentos', 'compartilhado/glossario'],
    motivo: 'o componente de ui/ não sabe do domínio'
  }
};

/**
 * Regra 7 (#475, ADR 0059): a lógica de uma tela (`paginas/<grupo>/<tela>.ts`, o que não é componente, diretiva, pipe nem
 * serviço) é função pura: sem Angular, sem rxjs e sem a API, para o Jest testá-la sem montar a tela.
 */
const LOGICA_DA_TELA = /^paginas\/(?:[^/]+\/)?[^/]+(?<!\.(?:component|directive|pipe|service))\.ts$/;
const PROIBIDOS_NA_LOGICA = [/^@angular\//, /^rxjs/, /^core\/api\.service$/];

/**
 * Regra 8 (#475, ADR 0059): os componentes de apresentação, que recebem por input() e avisam por output(). Não chamam a
 * API nem leem a rota: quem faz isso é o componente da tela. A lista é explícita porque o nome não basta (`contas-agendadas`
 * é uma tela com rota e começa com `contas-`). Componente de apresentação novo entra aqui.
 */
const APRESENTACAO: Record<string, string> = {
  'paginas/movimentacao/transacoes-lancamento.component.ts': 'janela "Novo lançamento" / "Editar transação"',
  'paginas/movimentacao/transacoes-transferencia.component.ts': 'janela "Transferência entre contas"',
  'paginas/movimentacao/transacoes-barra-edicao.component.ts': 'barra "Editando categorias"',
  'paginas/movimentacao/transacoes-barra-lote.component.ts': 'ações nas transações selecionadas',
  'paginas/movimentacao/transacoes-sugestoes.component.ts': 'janela "Sugestões de categoria"'
};
const PROIBIDOS_NA_APRESENTACAO = [/^core\/api\.service$/, /^@angular\/router$/];

/**
 * Exceções das regras 2 a 4, com o motivo. *Julgamento (#476):* parece estar no lugar errado, e não ser exceção de
 * verdade; fica aqui até alguém mover. As do AvisosService e dos avisos do sino saíram na #634 (core/ e layout/).
 */
const EXCECOES_DE_PASTA: Record<string, string> = {
  'layout/shell.component.ts -> paginas/manual/manual-conteudo':
    'o botão de ajuda leva à seção do Manual da tela (ajudaDaUrl); a função poderia morar no core'
};

/* ------------------------------------------------------------ o grafo */

const IMPORT = /(?:import|export)\s[^;]*?from\s+['"]([^'"]+)['"]|import\(\s*['"]([^'"]+)['"]\s*\)/g;

function arquivosTs(pasta: string): string[] {
  return readdirSync(pasta, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? arquivosTs(join(pasta, e.name)) : e.name.endsWith('.ts') ? [join(pasta, e.name)] : []
  );
}

const paraApp = (caminho: string) => relative(APP, caminho).split(sep).join('/');

function lerGrafo(): Grafo {
  const grafo: Grafo = {};
  for (const arquivo of arquivosTs(APP)) {
    const texto = readFileSync(arquivo, 'utf-8');
    const imports = [...texto.matchAll(IMPORT)].map((m) => {
      const alvo = m[1] ?? m[2];
      return alvo.startsWith('.') ? paraApp(normalize(join(dirname(arquivo), alvo))) : alvo;
    });
    grafo[paraApp(arquivo)] = { imports, texto };
  }
  return grafo;
}

/** As páginas com rota: os `paginas/<grupo>/x.component` (ou `paginas/x.component`, como o entrar) do app.routes.ts. */
function paginasComRota(grafo: Grafo): Set<string> {
  return new Set((grafo['app.routes.ts']?.imports ?? []).filter((i) => /^paginas\/(?:[^/]+\/)?[^/]+\.component$/.test(i)));
}

/* ------------------------------------------------------------ as regras */

function violacoes(grafo: Grafo): string[] {
  const v: string[] = [];
  const pasta = (caminho: string) => (caminho.includes('/') ? caminho.split('/')[0] : '');
  const rotas = paginasComRota(grafo);

  for (const [arquivo, { imports, texto }] of Object.entries(grafo)) {
    // Regra 1: o pacote de HTTP só no core e na configuração; o HttpClient injetado só no ApiService.
    if (imports.some((i) => i.startsWith('@angular/common/http')) && pasta(arquivo) !== 'core' && arquivo !== 'app.config.ts') {
      v.push(`regra 1 (HTTP só pelo ApiService): ${arquivo} importa @angular/common/http. O que fazer: chame a API pelo ApiService (core/api.service.ts).`);
    }
    if (/inject\(\s*HttpClient\s*\)/.test(texto) && !PODEM_INJETAR_HTTP[arquivo]) {
      v.push(`regra 1 (HTTP só pelo ApiService): ${arquivo} injeta o HttpClient. O que fazer: acrescente o método no ApiService e chame-o.`);
    }
    // Regras 2 a 4: o que cada pasta não importa.
    const regra = PASTAS_PROIBIDAS[pasta(arquivo)];
    for (const alvo of imports) {
      if (regra?.proibidas.includes(pasta(alvo)) && !EXCECOES_DE_PASTA[`${arquivo} -> ${alvo}`]) {
        v.push(`regra 2-4 (${regra.motivo}): ${arquivo} importa ${alvo}. O que fazer: mova o que é comum para o core (serviço, função) ou para o compartilhado (componente), ou registre em EXCECOES_DE_PASTA com o motivo.`);
      }
      // Regra 6: o compartilhado/ui/ não importa as pastas de assunto do compartilhado.
      const sub = Object.entries(SUBPASTAS_PROIBIDAS).find(([p]) => arquivo.startsWith(`${p}/`))?.[1];
      if (sub?.proibidas.some((p) => alvo.startsWith(`${p}/`))) {
        v.push(`regra 6 (${sub.motivo}): ${arquivo} importa ${alvo}. O que fazer: receba o dado pronto por input() (o ui/ só desenha), ou mova o componente para a pasta do assunto.`);
      }
      // Regra 7: a lógica da tela é função pura.
      if (LOGICA_DA_TELA.test(arquivo) && PROIBIDOS_NA_LOGICA.some((r) => r.test(alvo))) {
        v.push(`regra 7 (a lógica da tela é função pura, ADR 0059): ${arquivo} importa ${alvo}. O que fazer: deixe a chamada da API, a rota e o estado no componente da tela, e receba o dado pronto por parâmetro (a URL como { get(nome) }).`);
      }
      // Regra 8: o componente de apresentação não chama a API nem lê a rota.
      if (APRESENTACAO[arquivo] && PROIBIDOS_NA_APRESENTACAO.some((r) => r.test(alvo))) {
        v.push(`regra 8 (o componente de apresentação só recebe e avisa, ADR 0059): ${arquivo} importa ${alvo}. O que fazer: receba o dado por input() e avise por output(); a chamada da API e a rota ficam no componente da tela.`);
      }
      // Regra 5: uma página com rota não importa outra página com rota.
      const origem = arquivo.replace(/\.ts$/, '');
      if (rotas.has(origem) && rotas.has(alvo) && alvo !== origem) {
        v.push(`regra 5 (página não depende de outra página): ${arquivo} importa ${alvo}. O que fazer: leve para a outra tela pela rota (com o contexto na URL), ou mova o que é comum para o compartilhado.`);
      }
    }
  }
  for (const arquivo of Object.keys(APRESENTACAO)) {
    if (!grafo[arquivo]) {
      v.push(`regra 8: "${arquivo}" está em APRESENTACAO, mas não existe. O que fazer: corrija o caminho ou tire da tabela.`);
    }
  }
  for (const chave of Object.keys(EXCECOES_DE_PASTA)) {
    const [arquivo, alvo] = chave.split(' -> ');
    if (!grafo[arquivo]?.imports.includes(alvo)) {
      v.push(`regra 2-4: a exceção "${chave}" não acontece mais. O que fazer: tire de EXCECOES_DE_PASTA, para não voltar.`);
    }
  }
  return v.sort();
}

/* ------------------------------------------------------------ os testes */

describe('arquitetura do front-end (#476)', () => {
  it('o código de hoje segue as regras, com as exceções registradas', () => {
    const grafo = lerGrafo();
    expect(Object.keys(grafo).length).toBeGreaterThan(50);
    expect(paginasComRota(grafo).size).toBeGreaterThan(10);
    expect(violacoes(grafo)).toEqual([]);
  });

  describe('cada regra falha com um import proibido de mentira, e diz o que fazer', () => {
    const limpo = (): Grafo => ({
      'app.routes.ts': { imports: ['paginas/painel/painel.component', 'paginas/planejamento/metas.component'], texto: '' },
      'core/api.service.ts': { imports: ['@angular/common/http'], texto: 'inject(HttpClient)' },
      'compartilhado/ui/modal.component.ts': { imports: ['compartilhado/ui/icone.component', 'compartilhado/utilidades'], texto: '' },
      'layout/shell.component.ts': { imports: ['paginas/manual/manual-conteudo'], texto: '' },
      'paginas/painel/painel.component.ts': { imports: ['core/api.service'], texto: '' },
      'paginas/planejamento/metas.component.ts': { imports: ['paginas/planejamento/metas-plano'], texto: '' },
      ...Object.fromEntries(Object.keys(APRESENTACAO).map((a) => [a, { imports: ['compartilhado/ui/modal.component'], texto: '' }]))
    });
    const com = (arquivo: string, imports: string[], texto = ''): Grafo => ({ ...limpo(), [arquivo]: { imports, texto } });

    it('o grafo limpo passa', () => expect(violacoes(limpo())).toEqual([]));

    const casos: [string, Grafo, string][] = [
      ['HTTP fora do core', com('paginas/painel/painel.component.ts', ['@angular/common/http']),
        'regra 1 (HTTP só pelo ApiService): paginas/painel/painel.component.ts importa @angular/common/http'],
      ['HttpClient injetado fora do ApiService', com('core/novo.service.ts', ['@angular/common/http'], 'inject(HttpClient)'),
        'regra 1 (HTTP só pelo ApiService): core/novo.service.ts injeta o HttpClient'],
      ['core importa página', com('core/api.service.ts', ['@angular/common/http', 'paginas/painel/painel.component'], 'inject(HttpClient)'),
        'regra 2-4 (o core é a base: serviços e modelos): core/api.service.ts importa paginas/painel/painel.component'],
      ['compartilhado importa layout', com('compartilhado/graficos/grafico.component.ts', ['layout/shell.component']),
        'regra 2-4 (o componente reutilizável não sabe de uma tela): compartilhado/graficos/grafico.component.ts importa layout/shell.component'],
      ['layout importa página', com('layout/menu.ts', ['paginas/planejamento/metas.component']),
        'regra 2-4 (a casca da tela não depende de uma página): layout/menu.ts importa paginas/planejamento/metas.component'],
      ['página importa página', com('paginas/painel/painel.component.ts', ['paginas/planejamento/metas.component']),
        'regra 5 (página não depende de outra página): paginas/painel/painel.component.ts importa paginas/planejamento/metas.component'],
      ['ui importa assunto', com('compartilhado/ui/modal.component.ts', ['compartilhado/glossario/termo.component']),
        'regra 6 (o componente de ui/ não sabe do domínio): compartilhado/ui/modal.component.ts importa compartilhado/glossario/termo.component'],
      ['lógica da tela chama a API', com('paginas/planejamento/metas-plano.ts', ['core/api.service']),
        'regra 7 (a lógica da tela é função pura, ADR 0059): paginas/planejamento/metas-plano.ts importa core/api.service'],
      ['lógica da tela usa Angular', com('paginas/planejamento/metas-plano.ts', ['@angular/router']),
        'regra 7 (a lógica da tela é função pura, ADR 0059): paginas/planejamento/metas-plano.ts importa @angular/router'],
      ['apresentação chama a API', com('paginas/movimentacao/transacoes-lancamento.component.ts', ['core/api.service']),
        'regra 8 (o componente de apresentação só recebe e avisa, ADR 0059): paginas/movimentacao/transacoes-lancamento.component.ts importa core/api.service'],
      ['apresentação que sumiu', (() => { const g = limpo(); delete g['paginas/movimentacao/transacoes-transferencia.component.ts']; return g; })(),
        'regra 8: "paginas/movimentacao/transacoes-transferencia.component.ts" está em APRESENTACAO, mas não existe'],
      ['exceção que sarou', com('layout/shell.component.ts', []),
        'regra 2-4: a exceção "layout/shell.component.ts -> paginas/manual/manual-conteudo" não acontece mais']
    ];
    it.each(casos)('%s', (_nome, grafo, mensagem) => {
      const v = violacoes(grafo);
      expect(v).toHaveLength(1);
      expect(v[0].startsWith(mensagem)).toBe(true);
      expect(v[0]).toContain('O que fazer:');
    });
  });
});
