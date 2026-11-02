// Teste de arquitetura do front-end (#476), no estilo do ArchUnit do Java: quem pode importar quem em src/app.
//
// Só local, por decisão de 29/10/2026: fica fora de testes/ (o jest.config.js só procura lá), então o `npm test` e a
// CI não o rodam. Roda com `npm run test:arquitetura`. Os imports são lidos do código com uma expressão regular, como os
// outros testes que leem o código, sem dependência nova. Mudou a arquitetura de propósito? Mude a tabela aqui no mesmo
// PR, com o motivo.
import { readdirSync, readFileSync } from 'fs';
import { dirname, join, normalize, relative, sep } from 'path';

const APP = join(__dirname, '..', 'src', 'app');

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

/**
 * Exceções das regras 2 a 4, com o motivo. *Julgamento (#476):* as três parecem estar no lugar errado, e não ser
 * exceção de verdade; ficam aqui até alguém mover.
 */
const EXCECOES_DE_PASTA: Record<string, string> = {
  'core/status-sincronizacao.service.ts -> compartilhado/avisos.component':
    'o serviço abre o aviso padrão; o AvisosService poderia morar no core',
  'compartilhado/notificacoes.component.ts -> paginas/painel-avisos':
    'o sino das notificações mostra os avisos do Painel (#327); o cálculo dos avisos poderia morar no core',
  'compartilhado/notificacoes.component.ts -> paginas/painel-avisos.component':
    'o sino das notificações usa o componente dos avisos do Painel (#327)',
  'layout/shell.component.ts -> paginas/manual-conteudo':
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

/** As páginas com rota: os `paginas/x.component` do app.routes.ts. */
function paginasComRota(grafo: Grafo): Set<string> {
  return new Set((grafo['app.routes.ts']?.imports ?? []).filter((i) => /^paginas\/[^/]+\.component$/.test(i)));
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
      // Regra 5: uma página com rota não importa outra página com rota.
      const origem = arquivo.replace(/\.ts$/, '');
      if (rotas.has(origem) && rotas.has(alvo) && alvo !== origem) {
        v.push(`regra 5 (página não depende de outra página): ${arquivo} importa ${alvo}. O que fazer: leve para a outra tela pela rota (com o contexto na URL), ou mova o que é comum para o compartilhado.`);
      }
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
      'app.routes.ts': { imports: ['paginas/painel.component', 'paginas/metas.component'], texto: '' },
      'core/api.service.ts': { imports: ['@angular/common/http'], texto: 'inject(HttpClient)' },
      'core/status-sincronizacao.service.ts': { imports: ['compartilhado/avisos.component'], texto: '' },
      'compartilhado/notificacoes.component.ts': { imports: ['paginas/painel-avisos', 'paginas/painel-avisos.component'], texto: '' },
      'layout/shell.component.ts': { imports: ['paginas/manual-conteudo'], texto: '' },
      'paginas/painel.component.ts': { imports: ['core/api.service'], texto: '' },
      'paginas/metas.component.ts': { imports: ['paginas/metas-plano'], texto: '' }
    });
    const com = (arquivo: string, imports: string[], texto = ''): Grafo => ({ ...limpo(), [arquivo]: { imports, texto } });

    it('o grafo limpo passa', () => expect(violacoes(limpo())).toEqual([]));

    const casos: [string, Grafo, string][] = [
      ['HTTP fora do core', com('paginas/painel.component.ts', ['@angular/common/http']),
        'regra 1 (HTTP só pelo ApiService): paginas/painel.component.ts importa @angular/common/http'],
      ['HttpClient injetado fora do ApiService', com('core/novo.service.ts', ['@angular/common/http'], 'inject(HttpClient)'),
        'regra 1 (HTTP só pelo ApiService): core/novo.service.ts injeta o HttpClient'],
      ['core importa página', com('core/api.service.ts', ['@angular/common/http', 'paginas/painel.component'], 'inject(HttpClient)'),
        'regra 2-4 (o core é a base: serviços e modelos): core/api.service.ts importa paginas/painel.component'],
      ['compartilhado importa layout', com('compartilhado/grafico.component.ts', ['layout/shell.component']),
        'regra 2-4 (o componente reutilizável não sabe de uma tela): compartilhado/grafico.component.ts importa layout/shell.component'],
      ['layout importa página', com('layout/menu.ts', ['paginas/metas.component']),
        'regra 2-4 (a casca da tela não depende de uma página): layout/menu.ts importa paginas/metas.component'],
      ['página importa página', com('paginas/painel.component.ts', ['paginas/metas.component']),
        'regra 5 (página não depende de outra página): paginas/painel.component.ts importa paginas/metas.component'],
      ['exceção que sarou', com('layout/shell.component.ts', []),
        'regra 2-4: a exceção "layout/shell.component.ts -> paginas/manual-conteudo" não acontece mais']
    ];
    it.each(casos)('%s', (_nome, grafo, mensagem) => {
      const v = violacoes(grafo);
      expect(v).toHaveLength(1);
      expect(v[0].startsWith(mensagem)).toBe(true);
      expect(v[0]).toContain('O que fazer:');
    });
  });
});
