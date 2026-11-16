import { abrirMetas, meta, semTempoDeAviso } from '../apoio/metas';
import { METAS, plano } from '../apoio/plano-metas';

/**
 * #732 (ADR 0059): o assistente, o cartão e as janelas de meta e de aporte são componentes de apresentação; a tela é
 * quem grava e quem navega. Aqui, pelos controles de cada um (não pelos métodos da tela), que cada ação chega ao lugar
 * certo, e que as janelas editam uma cópia.
 */
describe('Regressão: componentes de apresentação de Metas (#732)', () => {
  beforeEach(() => localStorage.clear());

  const cartao = (raiz: HTMLElement, nome: string) =>
    [...raiz.querySelectorAll<HTMLElement>('.cartao-meta')].find((c) => c.querySelector('strong')!.textContent === nome)!;
  const botao = (dentro: ParentNode, texto: string) =>
    [...dentro.querySelectorAll<HTMLButtonElement>('button')].find((b) => b.textContent!.replace(/\s+/g, ' ').trim().includes(texto))!;

  it('o elemento do cartão é o próprio cartão: classe, papel de artigo e "fechada" acompanham a sanfona', async () => {
    const concluida = meta({ id: 2, nome: 'Notebook', concluida: true, percentual: 100 });
    const { raiz, atualizar } = abrirMetas([meta({ id: 1, nome: 'Viagem' }), concluida]);
    const viagem = cartao(raiz, 'Viagem');
    expect(viagem.tagName).toBe('APP-METAS-CARTAO');
    expect(viagem.getAttribute('role')).toBe('article');
    expect(viagem.classList).not.toContain('concluida');
    expect(cartao(raiz, 'Notebook').classList).toContain('concluida');
    botao(viagem, 'Viagem').click();
    await atualizar();
    expect(viagem.classList).toContain('fechada');
    expect(viagem.querySelector('.meta-conteudo')!.hasAttribute('inert')).toBe(true);
  });

  it('o plano mês a mês e o "Mostrar mais" são de cada cartão: abrir mais meses numa meta não mexe na outra', async () => {
    const { raiz, atualizar } = abrirMetas(METAS, plano());
    await atualizar();
    const [viagem, emergencia] = ['Viagem', 'Emergência'].map((n) => cartao(raiz, n));
    const linhas = (c: HTMLElement) => c.querySelectorAll('.tabela-plano tbody tr').length;
    const emergenciaAntes = linhas(emergencia);
    expect(linhas(viagem)).toBe(5);
    botao(viagem, 'Mostrar mais').click();
    await atualizar();
    expect([linhas(viagem), linhas(emergencia)]).toEqual([10, emergenciaAntes]);
    botao(viagem, 'Mostrar menos').click();
    botao(emergencia, 'Plano mês a mês').click();
    await atualizar();
    expect([linhas(viagem), linhas(emergencia)]).toEqual([5, 0]);
  });

  it('o link do assistente é da tela e continua um link (href); o cartão sem aviso de prazo nem cofrinho não tem links', async () => {
    // Os links de "Orçamentos", "visão anual" e "Reservas" (projetados no cartão) são conferidos em assistente-metas e metas-cofrinho.
    const { raiz, atualizar } = abrirMetas([meta({ id: 1, nome: 'Viagem', valorAlvo: 500, valorAtual: 0, valorInicial: 0, dataAlvo: null,
      reservaNome: null, saldoReserva: null, contaId: null, contaNome: null })], plano());
    await atualizar();
    const links = (dentro: ParentNode) => [...dentro.querySelectorAll('a')].map((a) => a.getAttribute('href'));
    expect(links(raiz.querySelector('.assistente-metas')!)).toEqual(['/projecao']);
    expect(links(cartao(raiz, 'Viagem'))).toEqual([]);
  });

  it('a janela da meta edita uma cópia: Cancelar não muda a meta da tela, e abrir de novo mostra o que estava gravado', async () => {
    const viagem = meta({ id: 1, nome: 'Viagem', valorAlvo: 10000 });
    const { tela, raiz, atualizar, formulario, atualizadas } = abrirMetas([viagem]);
    tela.editar(viagem);
    await atualizar();
    const nome = raiz.querySelector<HTMLInputElement>('input[name=nome]')!;
    nome.value = 'Outro nome';
    nome.dispatchEvent(new Event('input'));
    await atualizar();
    expect(formulario().edicao()!.dados.nome).toBe('Outro nome');
    expect(tela.formulario()!.dados.nome).toBe('Viagem');
    botao(raiz, 'Cancelar').click();
    await atualizar();
    expect(tela.formulario()).toBeNull();
    expect(atualizadas).toEqual([]);
    expect(tela.metas()[0].nome).toBe('Viagem');
    tela.editar(viagem);
    await atualizar();
    expect(raiz.querySelector<HTMLInputElement>('input[name=nome]')!.value).toBe('Viagem');
  });

  it('"Salvar" manda a cópia editada para a API e fecha a janela', async () => {
    const viagem = meta({ id: 1, nome: 'Viagem' });
    const { tela, raiz, atualizar, atualizadas, salvar } = abrirMetas([viagem]);
    tela.editar(viagem);
    await atualizar();
    const nome = raiz.querySelector<HTMLInputElement>('input[name=nome]')!;
    nome.value = 'Viagem ao Chile';
    nome.dispatchEvent(new Event('input'));
    await atualizar();
    salvar();
    expect(atualizadas).toHaveLength(1);
    expect(atualizadas[0]).toMatchObject({ id: 1, dto: { nome: 'Viagem ao Chile' } });
    expect(tela.formulario()).toBeNull();
  });

  it('o aporte vai com o valor digitado, e o negativo é uma retirada; sem valor, nada é registrado', async () => {
    const viagem = meta({ id: 4, nome: 'Viagem' });
    const { raiz, atualizar, aportes } = abrirMetas([viagem]);
    semTempoDeAviso();
    const abrir = async () => {
      raiz.querySelector<HTMLButtonElement>('button[aria-label="Aporte na meta Viagem"]')!.click();
      await atualizar();
    };
    const registrar = async (texto: string) => {
      const campo = raiz.querySelector<HTMLInputElement>('app-campo-valor[name="aporte"] input')!;
      campo.value = texto;
      campo.dispatchEvent(new Event('input'));
      await atualizar();
      botao(raiz, 'Registrar').click();
    };
    await abrir();
    expect(raiz.querySelector('[role=dialog]')!.getAttribute('aria-label')).toBe('Aporte · Viagem');
    botao(raiz, 'Registrar').click();
    expect(aportes).toEqual([]);
    await registrar('150');
    expect(aportes).toEqual([{ id: 4, valor: 150 }]);
    await atualizar();
    expect(raiz.querySelector('[role=dialog]')).toBeNull();
    await abrir();
    expect(raiz.querySelector<HTMLInputElement>('app-campo-valor[name="aporte"] input')!.value).toBe('');
    await registrar('-40');
    expect(aportes[1]).toEqual({ id: 4, valor: -40 });
  });
});
