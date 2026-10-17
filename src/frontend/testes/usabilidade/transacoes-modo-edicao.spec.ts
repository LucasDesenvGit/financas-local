import { TestBed } from '@angular/core/testing';
import { AvisosComponent, AvisosService } from '../../src/app/compartilhado/avisos.component';
import { abrirTransacoes, parametrosDaUrl } from '../apoio/transacoes';
import { abrirEdicao } from '../apoio/transacoes-edicao';

/**
 * Usabilidade do modo edição de categorias em Transações (#87). Nielsen 1: a troca não salva aparece na linha (em
 * texto, não só na cor — WCAG 1.4.1) e no contador; Nielsen 3: Esc, "Descartar" e "Desfazer"; Nielsen 5: confirmação
 * antes de mudar todas do filtro; WCAG 2.1.1/4.1.2: cada seletor tem nome com a transação; WCAG 2.2.1: o aviso com
 * "Desfazer" não some com o mouse ou o foco nele.
 */
describe('Usabilidade: modo edição de categorias', () => {
  afterEach(() => jest.restoreAllMocks());

  it('botão "Editar categorias" com aria-pressed; entrar grava ?editar=1 na URL', async () => {
    const t = await abrirTransacoes();
    const botao = Array.from(t.elemento.querySelectorAll<HTMLButtonElement>('button')).find((b) => b.textContent!.includes('Editar categorias'))!;
    expect(botao.getAttribute('aria-pressed')).toBe('false');
    expect(t.elemento.querySelector('select.select-categoria')).toBeNull();
    botao.click();
    await t.assentar();
    expect(botao.getAttribute('aria-pressed')).toBe('true');
    expect(botao.textContent).toContain('Sair da edição');
    expect(parametrosDaUrl(t.router)).toEqual({ editar: '1' });
  });

  it('cada linha tem um seletor de categoria com o nome da transação e a data', async () => {
    const t = await abrirEdicao();
    expect(t.selectDa(1).getAttribute('aria-label')).toBe('Categoria de PADARIA PAO QUENTE, 10/10');
    expect(t.selectDa(3).selectedOptions[0].textContent!.trim()).toBe('Sem categoria');
    expect(Array.from(t.selectDa(1).options).map((o) => o.textContent!.trim())).toContain('Automática (regras)');
  });

  it('troca pendente aparece na linha ("alterada") e no botão Salvar; Esc desfaz só aquela linha', async () => {
    const t = await abrirEdicao();
    expect(t.botao('Salvar alterações').disabled).toBe(true);
    await t.escolher(1, '20');
    await t.escolher(2, '20');
    const linha1 = t.elemento.querySelectorAll('tbody tr')[0];
    expect(linha1.querySelector('.etiqueta-alerta')!.textContent).toBe('alterada');
    expect(t.botao('Salvar alterações').textContent!.trim()).toBe('Salvar alterações (2)');
    expect(t.elemento.querySelector('.barra-edicao [role=status]')!.textContent).toContain('2 alteração(ões) não salva(s)');
    t.selectDa(1).dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await t.assentar();
    expect(t.botao('Salvar alterações').textContent!.trim()).toBe('Salvar alterações (1)');
    expect(t.selectDa(1).value).toBe('12');
  });

  it('"todas do filtro" pede confirmação com a quantidade, o período e a categoria', async () => {
    const t = await abrirEdicao('/transacoes?editar=1&inicio=2026-09-01&fim=2026-09-30', undefined, 240);
    const confirmar = jest.spyOn(window, 'confirm').mockReturnValue(false);
    t.tela.categoriaTodas = '20';
    t.tela.aplicarATodasDoFiltro();
    expect(confirmar).toHaveBeenCalledWith('Trocar a categoria das 240 transações do filtro (setembro/2026) para “Lazer”?');
    expect(t.lotes).toEqual([]);
  });

  it('o aviso com "Desfazer" fica enquanto o mouse ou o foco está nele', () => {
    jest.useFakeTimers();
    TestBed.configureTestingModule({});
    const avisos = TestBed.inject(AvisosService);
    const fixture = TestBed.createComponent(AvisosComponent);
    avisos.sucesso('2 transação(ões) atualizada(s).', { rotulo: 'Desfazer', executar: jest.fn() });
    fixture.detectChanges();
    const aviso = fixture.nativeElement.querySelector('.aviso') as HTMLElement;
    expect(aviso.querySelector('.aviso-acao')!.textContent).toBe('Desfazer');
    aviso.dispatchEvent(new Event('focusin'));
    jest.advanceTimersByTime(AvisosService.DURACAO_COM_ACAO * 3);
    expect(avisos.avisos()).toHaveLength(1);
    aviso.dispatchEvent(new Event('focusout'));
    jest.advanceTimersByTime(AvisosService.DURACAO_COM_ACAO + 1);
    expect(avisos.avisos()).toHaveLength(0);
    jest.useRealTimers();
  });
});
