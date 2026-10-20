import { expect, test as testeBase } from '@playwright/test';
import { ApiFalsa, ligarApiFalsa } from './api-falsa';

/** O que conta como "quebrou" em qualquer tela: erro no console, exceção, violação de CSP, erro mandado para a API. */
export interface Problemas {
  lista: string[];
  /** Ignora uma mensagem esperada num teste (ex.: o 401 de um login errado). */
  ignorar(trecho: string): void;
}

// "auto": todo teste liga a API falsa e confere os erros, mesmo sem pedir (um teste que esquecesse ficaria falando
// com o servidor de arquivos, que responde 599 a qualquer /api/).
export const test = testeBase.extend<{ api: ApiFalsa; problemas: Problemas }>({
  api: [async ({ page }, usar) => {
    await usar(await ligarApiFalsa(page));
  }, { auto: true }],
  problemas: [async ({ page, api }, usar) => {
    const lista: string[] = [];
    const ignorados: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error') lista.push(`console: ${msg.text()}`);
    });
    page.on('pageerror', (erro) => lista.push(`exceção: ${erro.message}`));
    // A violação de CSP nem sempre vira mensagem no console do Playwright (o script inline bloqueado do #157 não
    // virava): o evento do próprio navegador é a fonte certa. Escutado antes de a página carregar.
    await page.exposeFunction('__violacaoCsp', (texto: string) => lista.push(`CSP bloqueou: ${texto}`));
    await page.addInitScript(() => {
      document.addEventListener('securitypolicyviolation', (e) =>
        (window as unknown as { __violacaoCsp: (t: string) => void }).__violacaoCsp(`${e.violatedDirective} (${e.blockedURI || 'inline'})`));
    });
    await usar({ lista, ignorar: (trecho) => ignorados.push(trecho) });

    const sobrou = (texto: string) => !ignorados.some((t) => texto.includes(t));
    expect.soft(api.semSimulacao, 'chamadas da API sem simulação: acrescente em testes-navegador/api-falsa.ts').toEqual([]);
    expect.soft(api.errosDoSite, 'o site mandou erro para /api/erros/front-end').toEqual([]);
    expect(lista.filter(sobrou), 'erros no navegador').toEqual([]);
  }, { auto: true }]
});

export { expect };

/** As telas do menu, com o título que cada uma mostra (o h1). */
export const TELAS: { rota: string; titulo: RegExp }[] = [
  { rota: '/painel', titulo: /Painel/ },
  { rota: '/transacoes', titulo: /Transações/ },
  { rota: '/contas', titulo: /Contas e cartões/ },
  { rota: '/orcamentos', titulo: /Orçamentos/ },
  { rota: '/metas', titulo: /Metas/ },
  { rota: '/contas-a-pagar', titulo: /Contas a pagar/ },
  { rota: '/investimentos', titulo: /Investimentos/ },
  { rota: '/emprestimos', titulo: /Empréstimos/ },
  { rota: '/relatorios', titulo: /Relatórios/ },
  { rota: '/anual', titulo: /Visão anual/ },
  { rota: '/projecao', titulo: /Projeção/ },
  { rota: '/categorias', titulo: /Categorias/ },
  { rota: '/configuracoes', titulo: /Configurações/ },
  { rota: '/manual', titulo: /Manual/ }
];
