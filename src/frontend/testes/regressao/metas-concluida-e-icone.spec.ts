import { IconesMeta, iconeDaMeta } from '../../src/app/compartilhado/icones';
import { abrirMetas, meta } from '../apoio/metas';
import { plano } from '../apoio/plano-metas';

/**
 * Regressão da #316 (Metas):
 * - uma meta com R$ 200 de R$ 30.000 aparecia "concluída": a marcação ficava gravada depois de aumentar o alvo. Agora
 *   aumentar o alvo para acima do que já foi guardado reabre a meta, e a concluída sem chegar ao alvo diz "concluída por
 *   você" (não contradiz o 1%);
 * - sem meta em andamento, o assistente dizia "guardar R$ 0,00 dividido pela ordem de prioridade": agora diz que não há
 *   meta em andamento;
 * - "R$ 200,00de R$ 30.000,00": o espaço sumia entre as tags;
 * - ícone por meta (Nielsen 6) e o nome das cores para o leitor de tela (Nielsen 2).
 */
const RESERVA = meta({ id: 2, nome: 'RESERVA', valorAlvo: 30000, valorAtual: 200, valorInicial: 200, percentual: 1,
  concluida: true, contaId: null, contaNome: null, reservaNome: null, saldoReserva: null });

describe('Regressão: Metas, "concluída" com 1% e ícone (#316)', () => {
  beforeEach(() => localStorage.clear());

  it('concluída sem chegar ao alvo diz "concluída por você"; a que chegou continua "concluída"', async () => {
    const chegou = meta({ id: 3, nome: 'Notebook', valorAlvo: 5000, valorAtual: 5000, percentual: 100, concluida: true });
    const { raiz } = abrirMetas([RESERVA, chegou]);
    const etiquetas = [...raiz.querySelectorAll('.cartao-meta .etiqueta-sucesso')].map((e) => e.textContent!.trim());
    expect(etiquetas).toEqual(['concluída por você', 'concluída']);
  });

  it('o valor e o alvo têm espaço entre eles ("R$ 200,00 de R$ 30.000,00")', () => {
    const { raiz } = abrirMetas([RESERVA]);
    const linha = [...raiz.querySelectorAll('.cartao-meta p')].find((p) => p.textContent!.includes('30.000'))!;
    expect(linha.textContent!.replace(/\s+/g, ' ').trim()).toBe('R$ 200,00 de R$ 30.000,00');
  });

  it('aumentar o alvo de uma meta concluída para acima do que já foi guardado reabre a meta e diz por quê', async () => {
    const chegou = meta({ id: 3, nome: 'Notebook', valorAlvo: 200, valorAtual: 200, valorInicial: 200, percentual: 100,
      concluida: true, contaId: null, contaNome: null, reservaNome: null, saldoReserva: null });
    const { tela, raiz, atualizar } = abrirMetas([chegou]);
    tela.editar(chegou);
    await atualizar();
    tela.mudarAlvo(tela.form!, 30000);
    await atualizar();
    expect(tela.form!.concluida).toBe(false);
    expect(raiz.textContent).toContain('A meta voltou para o plano: o alvo novo é maior que o que já foi guardado.');
  });

  it('diminuir o alvo não mexe em "Concluída"; marcada sem chegar ao alvo, avisa quanto falta', async () => {
    const { tela, raiz, atualizar } = abrirMetas([RESERVA]);
    tela.editar(RESERVA);
    await atualizar();
    tela.mudarAlvo(tela.form!, 20000);
    await atualizar();
    expect(tela.form!.concluida).toBe(true);
    expect(raiz.textContent!.replace(/\s+/g, ' ')).toContain('Faltam R$ 19.800,00 para o alvo.');
  });

  it('sem meta em andamento, o assistente diz isso, e não "guardar R$ 0,00 dividido pela ordem de prioridade"', () => {
    const { raiz } = abrirMetas([RESERVA], plano({}));
    const assistente = raiz.querySelector('.assistente-metas')!.textContent!.replace(/\s+/g, ' ');
    expect(assistente).toContain('Nenhuma meta em andamento');
    expect(assistente).not.toContain('dividido pela ordem de prioridade');
  });

  it('o ícone escolhido aparece no card; sem ícone, o cofrinho', () => {
    expect(iconeDaMeta('viagem').nome).toBe('Viagem');
    expect(iconeDaMeta(null).chave).toBe('cofrinho');
    expect(iconeDaMeta('que-nao-existe').chave).toBe('cofrinho');
    // Chaves só com letras minúsculas e hífen (a API confere) e sem repetir.
    const chaves = IconesMeta.map((i) => i.chave);
    expect(chaves.every((c) => /^[a-z-]+$/.test(c))).toBe(true);
    expect(new Set(chaves).size).toBe(chaves.length);
    const { raiz } = abrirMetas([meta({ icone: 'viagem' })]);
    expect(raiz.querySelector('.cartao-meta .icone-meta app-icone')).not.toBeNull();
    expect(raiz.querySelector('.cartao-meta .icone-meta')!.getAttribute('aria-hidden')).toBe('true');
  });

  it('no formulário: o ícone escolhido fica marcado (aria-pressed) e as cores têm nome', async () => {
    const viagem = meta({ icone: 'viagem', cor: '#92c8f0' });
    const { tela, raiz, atualizar } = abrirMetas([viagem]);
    tela.editar(viagem);
    await atualizar();
    const marcados = [...raiz.querySelectorAll('.opcao-icone[aria-pressed="true"]')].map((b) => b.getAttribute('aria-label'));
    expect(marcados).toEqual(['Ícone Viagem']);
    const cor = raiz.querySelector('.cor[aria-pressed="true"]')!;
    expect(cor.getAttribute('aria-label')).toBe('Cor azul');
    (raiz.querySelector<HTMLButtonElement>('.opcao-icone[aria-label="Ícone Casa"]')!).click();
    await atualizar();
    expect(tela.form!.icone).toBe('casa');
  });
});
