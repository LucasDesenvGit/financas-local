import { TestBed } from '@angular/core/testing';
import { SeloServicoComponent } from '../../src/app/compartilhado/selo-servico.component';
import { IconeCobrancaPadrao, IconesCategoria } from '../../src/app/compartilhado/icones';
import { SERVICOS, servicoDe } from '../../src/app/compartilhado/servicos';

/**
 * #265: o ícone de cada cobrança em Contas a pagar. Serviço conhecido pelo texto → selo da marca (como os bancos,
 * ADR 0024); senão, o ícone da categoria; sem categoria conhecida, o recibo.
 */
describe('Regressão: selo de cada cobrança (#265)', () => {
  it('reconhece os serviços do print da issue', () => {
    expect(servicoDe('TIM POS')?.chave).toBe('tim');
    expect(servicoDe('CLUBE*ESFERA')?.chave).toBe('esfera');
    expect(servicoDe('IFD*iFood')?.chave).toBe('ifood');
    // O mais específico vem antes: é YouTube, e não Google.
    expect(servicoDe('Google YouTubePremium')?.chave).toBe('youtube');
    expect(servicoDe('GOOGLE *Google One')?.chave).toBe('google');
  });

  it('reconhece as marcas mais comuns no Brasil pelos textos do extrato', () => {
    const casos: [string, string][] = [
      ['SPOTIFY P1A2B3', 'spotify'], ['TIM S/A', 'tim'], ['VIVO FIBRA', 'vivo'], ['TELEFONICA BRASIL', 'vivo'],
      ['OI S.A.', 'oi'], ['OI FIBRA', 'oi'], ['CLARO SA', 'claro'], ['NET SERVICOS', 'claro'], ['IFD*IFOOD.COM', 'ifood'],
      ['UBER *TRIP', 'uber'], ['UBER* EATS', 'ubereats'], ['99 TAXI', '99'], ['99*99APP', '99'], ['99FOOD', '99'],
      ['AMAZON.COM.BR', 'amazon'], ['AMZN MKTP', 'amazon'], ['AMAZON PRIME', 'primevideo'],
      ['MERCADOLIVRE*MERCADOLIVRE', 'mercadolivre'], ['MP*MERCADOLIVRE', 'mercadolivre'], ['MELI+ ASSINATURA', 'mercadolivre'],
      ['RAPPI BRASIL', 'rappi'], ['SEM PARAR', 'semparar'], ['SABESP', 'sabesp'], ['ENEL DISTRIBUICAO SP', 'enel'],
      ['UNIMED SAO PAULO', 'unimed'], ['SMARTFIT', 'smartfit'], ['DISNEY PLUS', 'disney'], ['GLOBOPLAY', 'globoplay'],
      ['YOUTUBE MUSIC', 'youtubemusic'], ['GOOGLE GEMINI', 'gemini'], ['APPLE.COM/BILL', 'apple']
    ];
    for (const [texto, chave] of casos) expect([texto, servicoDe(texto)?.chave]).toEqual([texto, chave]);
  });

  it('#269: Uber pelo processador (DL*UberRides), Mercado Pago, Pix, Cobasi, Petz, Petlove e Extra', () => {
    const casos: [string, string][] = [
      ['DL*UberRides', 'uber'], ['UBER *TRIP', 'uber'], ['UBERTRIP', 'uber'], ['UBER ONE', 'uber'], ['DL*UberEats', 'ubereats'],
      ['MP*MP', 'mercadopago'], ['MP*JULIOCEZARSIL', 'mercadopago'], ['MERCADO PAGO*LOJA', 'mercadopago'],
      ['MP*MERCADOLIVRE', 'mercadolivre'],                        // é do Mercado Livre, não do Mercado Pago
      ['PIX ENVIADO PESSOA EXEMPLO', 'pix'], ['PIX RECEBIDO', 'pix'], ['PIX NETFLIX.COM', 'netflix'],   // marca antes do Pix
      ['BILHUNICO', 'bilheteunico'], ['BILHETE UNICO SPTRANS', 'bilheteunico'], ['COBASI', 'cobasi'], ['PETZ', 'petz'], ['PETLOVE*ASSINATURA', 'petlove'], ['Petlove Saud*Petlove S', 'petlove'],
      ['EXTRA HIPER 123', 'extra'], ['MERCADO EXTRA', 'extra'], ['EXTRA.COM.BR', 'extra']
    ];
    for (const [texto, chave] of casos) expect([texto, servicoDe(texto)?.chave]).toEqual([texto, chave]);
    // "extra" e "MP" soltos em outras palavras não viram marca.
    expect(servicoDe('EXTRATO MENSAL')).toBeNull();
    expect(servicoDe('PAGTO HORA EXTRA')).toBeNull();
    expect(servicoDe('AMPLA ENERGIA')).toBeNull();
    expect(servicoDe('POSTO UBERLANDIA')).toBeNull();
  });

  it('palavra comum não vira marca: "OI" só sozinho, Méliuz não é Mercado Livre, "LIGHT" só com "SERV"', () => {
    expect(servicoDe('COMPRA OIAPOQUE')).toBeNull();
    expect(servicoDe('MELIUZ CASHBACK')).toBeNull();
    expect(servicoDe('COCA LIGHT')).toBeNull();
    expect(servicoDe('LIGHT SERVICOS DE ELETRICIDADE')?.chave).toBe('light');
    expect(servicoDe('CANETA AZUL')).toBeNull();
  });

  it('termo curto só casa com a palavra inteira; o texto da cobrança vale antes do nome', () => {
    expect(servicoDe('TIMBRE ESCOLA')).toBeNull();
    expect(servicoDe('POSTO UBERLANDIA')).toBeNull();
    expect(servicoDe('UBER *TRIP')?.chave).toBe('uber');
    expect(servicoDe('NETFLIX.COM', 'Minha assinatura de Spotify')?.chave).toBe('netflix');
    // #274: entre os textos vale a marca mais específica (a que vem antes na lista), não a do primeiro texto.
    expect(servicoDe('GOOGLE', null, 'Google YouTubePremium')?.chave).toBe('youtube');
    expect(servicoDe('PIX ENVIADO', null, 'Netflix')?.chave).toBe('netflix');
    expect(servicoDe('GOOGLE', null, 'Google One')?.chave).toBe('google');
    expect(servicoDe(null, 'Spotify Família')?.chave).toBe('spotify');
    expect(servicoDe('ALUGUEL', 'Condomínio')).toBeNull();
  });

  it('todo serviço tem desenho (Simple Icons) ou sigla, e cor de fundo e do desenho', () => {
    for (const s of SERVICOS) {
      expect(Boolean(s.visual.caminho) !== Boolean(s.visual.sigla)).toBe(true);
      expect(s.visual.fundo).toMatch(/^#[0-9A-F]{6}$/i);
      expect(s.visual.cor).toMatch(/^#[0-9A-F]{6}$/i);
    }
  });

  function selo(entradas: Record<string, string | null>) {
    const fixture = TestBed.createComponent(SeloServicoComponent);
    for (const [nome, valor] of Object.entries(entradas)) fixture.componentRef.setInput(nome, valor);
    fixture.detectChanges();
    return { el: fixture.nativeElement as HTMLElement, tela: fixture.componentInstance };
  }

  it('serviço conhecido desenha o selo da marca, escondido do leitor de tela', () => {
    const { el } = selo({ texto: 'YOUTUBEPREMIUM', nome: 'YouTube' });
    const svg = el.querySelector('svg.logo-banco')!;
    expect(svg.getAttribute('aria-hidden')).toBe('true');
    expect(svg.querySelector('rect')!.getAttribute('fill')).toBe('#FF0000');
  });

  it('sem serviço, o ícone da categoria (sem acento); categoria desconhecida, o recibo', () => {
    expect(selo({ nome: 'Escola', categoria: 'Educação' }).tela.iconeCategoria()).toBe(IconesCategoria['EDUCACAO']);
    expect(selo({ nome: 'Aluguel', categoria: 'Moradia' }).tela.iconeCategoria()).toBe(IconesCategoria['MORADIA']);
    expect(selo({ nome: 'Aluguel', categoria: 'Minha categoria' }).tela.iconeCategoria()).toBe(IconeCobrancaPadrao);
    const { el } = selo({ nome: 'Escola', categoria: 'Educação' });
    expect(el.querySelector('svg.logo-banco')).toBeNull();
    expect(el.querySelector('.logo-generico')!.getAttribute('aria-hidden')).toBe('true');
  });
});
