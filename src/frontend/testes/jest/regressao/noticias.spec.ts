import { FonteNoticias, Noticia } from '../../../src/app/core/modelos';
import { dataUtc, filtrarPorFonte, quandoFoiPublicada, situacaoDaFonte } from '../../../src/app/paginas/noticias/noticias';

/** #10 (ADR 0067): as regras da tela Notícias (buscar e ler as fontes é da API, testado no Go). */
describe('Notícias (#10)', () => {
  const agora = new Date('2026-11-14T15:00:00Z');
  const noticia = (fonte: string, publicadaEm: string | null): Noticia =>
    ({ fonte, fonteNome: fonte.toUpperCase(), titulo: 't', link: 'https://g1.globo.com/x', publicadaEm });

  it('a data da API é UTC, com ou sem o Z', () => {
    expect(dataUtc('2026-11-14T12:30:00').toISOString()).toBe('2026-11-14T12:30:00.000Z');
    expect(dataUtc('2026-11-14T12:30:00Z').toISOString()).toBe('2026-11-14T12:30:00.000Z');
    expect(dataUtc('2026-11-14T12:30:00-03:00').toISOString()).toBe('2026-11-14T15:30:00.000Z');
  });

  it('filtra pela fonte; vazio é todas', () => {
    const lista = [noticia('g1', null), noticia('bbc', null)];
    expect(filtrarPorFonte(lista, '')).toBe(lista);
    expect(filtrarPorFonte(lista, 'bbc')).toEqual([lista[1]]);
  });

  it('a situação da fonte e quando a notícia saiu, em palavras', () => {
    const f: FonteNoticias = { id: 'g1', nome: 'G1', atualizadoEm: '2026-11-14T14:50:00', erro: null };
    expect(situacaoDaFonte(f, agora)).toBe('G1: atualizado há 10 min');
    expect(situacaoDaFonte({ ...f, erro: 'G1: respondeu 500' }, agora)).toBe('G1: atualizado há 10 min; a última tentativa falhou');
    expect(situacaoDaFonte({ ...f, atualizadoEm: null }, agora)).toBe('G1: ainda não atualizado');
    expect(quandoFoiPublicada(noticia('g1', '2026-11-14T13:00:00'), agora)).toBe('há 2 h');
    expect(quandoFoiPublicada(noticia('g1', null), agora)).toBe('');
  });
});
