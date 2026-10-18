import { TestBed } from '@angular/core/testing';
import { LogoBancoComponent } from '../../src/app/compartilhado/logo-banco.component';
import { abrirContas, fatura } from '../apoio/contas';

/**
 * Usabilidade de Contas e cartões (ADR 0024). Shneiderman: resumo primeiro (saldo, faturas do mês, dívida, limite),
 * depois cada conta, e o histórico de faturas sob demanda; Progressive Disclosure (NN/g): histórico recolhido;
 * Nielsen 2/10: fatura do mês × dívida total com ⓘ do glossário; Nielsen 6: o selo do banco ajuda a reconhecer a
 * conta; WCAG 1.1.1: selo decorativo quando o nome do banco já está escrito ao lado; WCAG 4.1.2: botões com estado.
 */
describe('Usabilidade: Contas e cartões', () => {
  it('o resumo vem antes das contas, com fatura do mês e dívida total separadas e explicadas', async () => {
    const t = await abrirContas();
    const resumo = t.raiz.querySelector('.kpis-contas')!;
    expect(resumo.compareDocumentPosition(t.raiz.querySelector('#titulo-contas')!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    const rotulos = Array.from(resumo.querySelectorAll('.kpi > span')).map((s) => s.textContent!.replace('ⓘ', '').trim());
    expect(rotulos[0]).toBe('Saldo em contas');
    expect(rotulos[1]).toMatch(/^Faturas de /);
    expect(rotulos[2]).toBe('Dívida total nos cartões');
    expect(rotulos[3]).toBe('Limite disponível');
    expect(resumo.querySelector('[aria-label="O que é Fatura do mês?"]')).not.toBeNull();
    expect(resumo.querySelector('[aria-label="O que é Dívida total nos cartões?"]')).not.toBeNull();
  });

  it('o selo do banco é decorativo e o nome do banco vem escrito', async () => {
    const t = await abrirContas();
    const contaNubank = t.raiz.querySelector('.cartao-conta')!;
    expect(contaNubank.querySelector('app-logo-banco svg')!.getAttribute('aria-hidden')).toBe('true');
    expect(contaNubank.textContent).toContain('Nubank · Conta corrente');
  });

  it('cartão mostra "Fatura de <mês>" e "Dívida total" (com ⓘ) lado a lado, e o melhor dia de compra', async () => {
    const t = await abrirContas();
    const cartao = t.cartao(2);
    expect(cartao.querySelector('.valor-fatura-mes span')!.textContent).toMatch(/^Fatura de /);
    expect(cartao.querySelector('.valor-divida [aria-label="O que é Dívida total do cartão?"]')).not.toBeNull();
    expect(cartao.textContent!.replace(/\s+/g, ' ')).toMatch(/fecha dia 3 · vence dia 10 · melhor dia de compra: 4\s*ⓘ/);
  });

  it('histórico de faturas começa recolhido, num botão com aria-expanded, e só carrega ao abrir', async () => {
    const t = await abrirContas('/contas', undefined, { 2: [fatura()] });
    const botao = t.cartao(2).querySelector<HTMLButtonElement>('.botao-historico')!;
    expect(botao.tagName).toBe('BUTTON');
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    expect(t.pedidas).toEqual([]);
    botao.click();
    await t.atualizar();
    expect(botao.getAttribute('aria-expanded')).toBe('true');
    expect(document.getElementById(botao.getAttribute('aria-controls')!)!.querySelector('table caption')!.textContent)
      .toContain('Faturas de Ultravioleta');
    expect(t.pedidas).toEqual([2]);
  });

  it('no formulário de cartão: "Banco" com o detectado e "Conta do cartão", com ajuda ligada', async () => {
    const t = await abrirContas();
    t.tela.editar(t.tela.cartoes().find((c) => c.id === 3)!);
    await t.atualizar();
    const banco = t.raiz.querySelector<HTMLSelectElement>('select[name=banco]')!;
    expect(banco.closest('label')!.textContent).toContain('Banco');
    expect(banco.getAttribute('aria-describedby')).toBe('ajuda-banco');
    const vinculada = t.raiz.querySelector<HTMLSelectElement>('select[name=contaVinculada]')!;
    expect(vinculada.closest('label')!.textContent).toContain('Conta do cartão');
    // "Nubank conta" já diz o banco: não repete ("Nubank conta · Nubank").
    expect(Array.from(vinculada.options).map((o) => o.textContent!.trim())).toEqual(['Nenhuma', 'Nubank conta']);
  });

  it('selo sozinho (com rótulo) vira imagem com nome; banco desconhecido mostra ícone neutro', () => {
    TestBed.configureTestingModule({});
    const f = TestBed.createComponent(LogoBancoComponent);
    f.componentRef.setInput('codigo', 'itau');
    f.componentRef.setInput('rotulo', 'Itaú');
    f.detectChanges();
    const svg = f.nativeElement.querySelector('svg')!;
    expect(svg.getAttribute('role')).toBe('img');
    expect(svg.getAttribute('aria-label')).toBe('Itaú');
    expect(svg.querySelector('text')!.textContent!.trim()).toBe('itaú');
    f.componentRef.setInput('codigo', 'banco-que-nao-existe');
    f.detectChanges();
    expect(f.nativeElement.querySelector('.logo-generico')).not.toBeNull();
  });
});
