import { codigosDoApp } from '../apoio/codigo';

/**
 * Regressão da varredura de ponta a ponta (20/10/2026): editar uma transação, uma categoria ou uma regra de categorização
 * e reutilizar um resultado da calculadora só funcionavam com o mouse (clique numa linha <tr>/<li>, que não recebe foco).
 * WCAG 2.2 — 2.1.1 Keyboard (A): todo clique num elemento que não é botão nem link precisa de um <button>/<a> no
 * mesmo template que chama a mesma ação. Fundos de modal e da gaveta ficam de fora: o Esc já fecha.
 */
const SEM_EQUIVALENTE_OK = new Set(['emit', 'fecharGaveta']);

describe('Regressão: clique em linha ou bloco tem o mesmo caminho pelo teclado (WCAG 2.1.1)', () => {
  it('cada (click) em tr/li/div/span/td/article tem um botão ou link com a mesma ação', () => {
    const faltando: string[] = [];
    // O template em arquivo próprio vem junto com o .ts (#475).
    for (const { arquivo, texto: fonte } of codigosDoApp()) {
      for (const m of fonte.matchAll(/<(tr|li|div|span|td|article|section|p|header)\b[^>]*?\(click\)="([^"]+)"/g)) {
        const chamadas = [...m[2].matchAll(/(\w+)\(/g)].map((c) => c[1]).filter((n) => n !== 'stopPropagation');
        const acao = chamadas.at(-1);
        if (!acao || SEM_EQUIVALENTE_OK.has(acao)) continue;
        const equivalente = new RegExp(`<(button|a)\\b[^>]*?\\(click\\)="[^"]*\\b${acao}\\(`);
        if (!equivalente.test(fonte)) faltando.push(`${arquivo}: <${m[1]}> ${acao}()`);
      }
    }
    expect(faltando).toEqual([]);
  });
});
