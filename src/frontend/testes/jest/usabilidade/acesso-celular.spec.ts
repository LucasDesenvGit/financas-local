import { TestBed } from '@angular/core/testing';
import { Observable, of, throwError } from 'rxjs';
import { HttpErrorResponse } from '@angular/common/http';
import { ApiService } from '../../../src/app/core/api.service';
import { AvisosService } from '../../../src/app/core/avisos.service';
import { SituacaoCelular } from '../../../src/app/core/modelos';
import { ConfirmacaoService, PedidoConfirmacao } from '../../../src/app/compartilhado/ui/confirmacao.service';
import { AcessoCelularComponent } from '../../../src/app/paginas/ajustes/acesso-celular.component';
import { MARGEM_DO_QR, abertoPelaRede, desenharQr, enderecoDaAutoridade, ipInicial } from '../../../src/app/paginas/ajustes/acesso-celular';
import { descreverEvento } from '../../../src/app/paginas/ajustes/auditoria';

const wifi = { ip: '192.168.0.10', interface: 'Wi-Fi', rede: '192.168.0.0/24' };
const cabo = { ip: '10.0.0.20', interface: 'Ethernet', rede: '10.0.0.0/8' };

function situacao(campos: Partial<SituacaoCelular> = {}): SituacaoCelular {
  return { ligado: false, ip: null, endereco: null, ipIndisponivel: false, enderecos: [wifi, cabo], impressaoDigital: null,
    avisos: [], ...campos };
}

const ligada = situacao({ ligado: true, ip: '192.168.0.10', endereco: 'https://192.168.0.10:5443',
  impressaoDigital: 'AB:CD:EF', avisos: ['O Windows marcou esta rede como Pública.'] });

/** Abre o quadro com a API falsa; `confirma` é a resposta da janela de confirmação. */
async function abrir(inicial: SituacaoCelular, opcoes: { confirma?: boolean; ligar?: Observable<SituacaoCelular> } = {}) {
  const ligados: string[] = [];
  const pedidos: PedidoConfirmacao[] = [];
  const avisos: string[] = [];
  let desligados = 0;
  TestBed.configureTestingModule({
    providers: [
      { provide: ApiService, useValue: {
        situacaoCelular: () => of(inicial),
        ligarCelular: (ip: string) => { ligados.push(ip); return opcoes.ligar ?? of(ligada); },
        desligarCelular: () => { desligados++; return of(situacao()); }
      } },
      { provide: ConfirmacaoService, useValue: { confirmar: (p: PedidoConfirmacao) => { pedidos.push(p); return Promise.resolve(opcoes.confirma ?? true); } } },
      { provide: AvisosService, useValue: { sucesso: (t: string) => avisos.push('ok: ' + t), erro: (t: string) => avisos.push('erro: ' + t) } }
    ]
  });
  const fixture = TestBed.createComponent(AcessoCelularComponent);
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
  const raiz = fixture.nativeElement as HTMLElement;
  const assentar = async () => { await fixture.whenStable(); fixture.detectChanges(); };
  const botao = (texto: string) => Array.from(raiz.querySelectorAll('button')).find((b) => b.textContent!.trim() === texto)!;
  return { fixture, raiz, assentar, botao, ligados, pedidos, avisos, desligados: () => desligados };
}

const texto = (e: Element | null) => (e?.textContent ?? '').replace(/\s+/g, ' ').trim();

describe('Usabilidade: acesso pelo celular (#328)', () => {
  it('desligado: diz o que é, o risco (Nielsen 2 e 5) e oferece os IPs com rótulo visível (WCAG 3.3.2)', async () => {
    const t = await abrir(situacao());
    expect(texto(t.raiz.querySelector('h2'))).toBe('Acesso pelo celular');
    expect(texto(t.raiz)).toContain('Desligado, o site só abre neste computador.');
    expect(texto(t.raiz.querySelector('.aviso-inline'))).toContain('use uma senha forte');
    const rotulo = t.raiz.querySelector('label')!;
    expect(texto(rotulo)).toMatch(/^Endereço deste computador na rede/);
    const opcoes = Array.from(rotulo.querySelectorAll('option')).map(texto);
    expect(opcoes).toEqual(['192.168.0.10 (Wi-Fi)', '10.0.0.20 (Ethernet)']);
    expect(rotulo.querySelector('select')!.getAttribute('aria-describedby')).toBe('ajuda-ip-celular');
    expect(t.botao('Ligar o acesso pelo celular').disabled).toBe(false);
  });

  it('ligar pede confirmação com o IP e o que muda; cancelado, não liga', async () => {
    const t = await abrir(situacao(), { confirma: false });
    t.botao('Ligar o acesso pelo celular').click();
    await t.assentar();
    expect(t.pedidos.length).toBe(1);
    expect(t.pedidos[0].mensagem).toContain('192.168.0.10');
    expect(t.pedidos[0].mensagem).toContain('a sua senha passa a ser a proteção');
    expect(t.pedidos[0].confirmar).toBe('Ligar');
    expect(t.ligados).toEqual([]);
  });

  it('ligado: o endereço, os avisos, os passos do Android e do iPhone e a impressão digital', async () => {
    const t = await abrir(situacao(), { confirma: true });
    t.botao('Ligar o acesso pelo celular').click();
    await t.assentar();
    expect(t.ligados).toEqual(['192.168.0.10']);
    expect(t.avisos).toEqual(['ok: Acesso pelo celular ligado.']);
    expect(texto(t.raiz.querySelector('[role=status]'))).toContain('https://192.168.0.10:5443');
    expect(texto(t.raiz.querySelector('.aviso-inline'))).toBe('O Windows marcou esta rede como Pública.');
    const passos = Array.from(t.raiz.querySelectorAll('ol li')).map(texto);
    expect(passos[0]).toContain('https://192.168.0.10:5443/api/celular/autoridade');
    expect(passos[1]).toMatch(/^Android:/);
    expect(passos[2]).toMatch(/^iPhone:.*Ajustes de Confiança de Certificados/);
    expect(texto(t.raiz.querySelector('.impressao-digital'))).toBe('AB:CD:EF');
    const baixar = t.raiz.querySelector<HTMLAnchorElement>('a[download]')!;
    expect(baixar.getAttribute('href')).toBe('/api/celular/autoridade');
    expect(texto(baixar)).toBe('Baixar o certificado neste computador');
  });

  it('#677: ligado, um QR code para o certificado e outro para o site, com legenda e nome para o leitor de tela (WCAG 1.1.1)', async () => {
    const t = await abrir(ligada);
    const qrs = Array.from(t.raiz.querySelectorAll('.qrs-celular figure'));
    expect(qrs.map((f) => texto(f.querySelector('figcaption')))).toEqual(['1. Certificado', '4. O site']);
    const imagens = qrs.map((f) => f.querySelector('svg')!);
    expect(imagens.map((i) => i.getAttribute('role'))).toEqual(['img', 'img']);
    expect(imagens[0].getAttribute('aria-label')).toContain('certificado');
    expect(imagens[1].getAttribute('aria-label')).toContain('site');
    expect(imagens[0].querySelector('path')!.getAttribute('d')).toBe(desenharQr('https://192.168.0.10:5443/api/celular/autoridade').caminho);
    expect(imagens[1].querySelector('path')!.getAttribute('d')).toBe(desenharQr('https://192.168.0.10:5443').caminho);
  });

  it('#677: os botões e o link de baixar têm ícone, decorativo (aria-hidden), e o texto continua sendo o nome', async () => {
    const desligado = await abrir(situacao());
    expect(desligado.botao('Ligar o acesso pelo celular').querySelector('svg[aria-hidden=true]')).not.toBeNull();
    TestBed.resetTestingModule();
    const t = await abrir(ligada);
    expect(t.botao('Desligar o acesso pelo celular').querySelector('svg[aria-hidden=true]')).not.toBeNull();
    expect(t.raiz.querySelector('a[download] svg[aria-hidden=true]')).not.toBeNull();
  });

  it('desligado, sem QR code', async () => {
    const t = await abrir(situacao());
    expect(t.raiz.querySelector('.qrs-celular')).toBeNull();
  });

  it('o IP escolhido antes que saiu do computador aparece, e a lista começa no primeiro de agora', async () => {
    const t = await abrir(situacao({ ip: '192.168.1.50', ipIndisponivel: true }));
    expect(texto(t.raiz)).toContain('estava ligado em 192.168.1.50, que não está mais neste computador');
    expect(t.fixture.componentInstance.ipEscolhido).toBe('192.168.0.10');
  });

  it('sem rede de casa, diz o que fazer em vez de uma lista vazia', async () => {
    const t = await abrir(situacao({ enderecos: [] }));
    expect(texto(t.raiz.querySelector('.vazio'))).toContain('não está numa rede de casa agora');
    expect(t.raiz.querySelector('select')).toBeNull();
  });

  it('desligar pelo computador é na hora, sem confirmação', async () => {
    const t = await abrir(ligada);
    t.botao('Desligar o acesso pelo celular').click();
    await t.assentar();
    expect(t.pedidos).toEqual([]);
    expect(t.desligados()).toBe(1);
    expect(t.avisos).toEqual(['ok: Acesso pelo celular desligado. O site só abre neste computador.']);
    expect(t.botao('Ligar o acesso pelo celular')).toBeTruthy();
  });

  it('falha ao ligar mostra a mensagem da API e libera o botão', async () => {
    const erro = new HttpErrorResponse({ status: 409, error: { mensagem: 'Não deu para abrir a porta no firewall do Windows.' } });
    const t = await abrir(situacao(), { ligar: throwError(() => erro) });
    t.botao('Ligar o acesso pelo celular').click();
    await t.assentar();
    expect(t.avisos).toEqual(['erro: Não deu para abrir a porta no firewall do Windows.']);
    expect(t.botao('Ligar o acesso pelo celular').disabled).toBe(false);
  });
});

describe('Regras do acesso pelo celular (#328)', () => {
  it('aberto pela rede é tudo que não é o próprio computador', () => {
    expect(abertoPelaRede('127.0.0.1')).toBe(false);
    expect(abertoPelaRede('localhost')).toBe(false);
    expect(abertoPelaRede('LOCALHOST')).toBe(false);
    expect(abertoPelaRede('[::1]')).toBe(false);
    expect(abertoPelaRede('192.168.0.10')).toBe(true);
  });

  it('o IP marcado é o escolhido antes, se ainda existe; senão o primeiro; sem rede, vazio', () => {
    expect(ipInicial(situacao({ ip: '10.0.0.20' }))).toBe('10.0.0.20');
    expect(ipInicial(situacao({ ip: '192.168.1.50' }))).toBe('192.168.0.10');
    expect(ipInicial(situacao())).toBe('192.168.0.10');
    expect(ipInicial(situacao({ enderecos: [] }))).toBe('');
  });

  it('o endereço do certificado é o do site ligado com o caminho da autoridade', () => {
    expect(enderecoDaAutoridade(ligada)).toBe('https://192.168.0.10:5443/api/celular/autoridade');
    expect(enderecoDaAutoridade(situacao())).toBeNull();
  });

  it('#677: o QR code tem a margem de 4 módulos e os três quadrados de canto (o que a câmera procura)', () => {
    const { lado, caminho } = desenharQr('https://192.168.0.10:5443/api/celular/autoridade');
    const n = lado - 2 * MARGEM_DO_QR;
    expect((n - 21) % 4).toBe(0); // os tamanhos do QR: 21, 25, 29... módulos
    const escuros = new Set(Array.from(caminho.matchAll(/M(\d+) (\d+)h1v1h-1z/g), (m) => `${m[1]},${m[2]}`));
    expect(escuros.size).toBe(caminho.split('M').length - 1);
    for (const [x, y] of [[0, 0], [n - 7, 0], [0, n - 7]]) {
      // A borda de 7×7 do quadrado de canto é escura, o anel de dentro é claro e o miolo 3×3 é escuro.
      for (let i = 0; i < 7; i++) {
        for (const [cx, cy] of [[x + i, y], [x + i, y + 6], [x, y + i], [x + 6, y + i]]) {
          expect(escuros.has(`${cx + MARGEM_DO_QR},${cy + MARGEM_DO_QR}`)).toBe(true);
        }
      }
      expect(escuros.has(`${x + 1 + MARGEM_DO_QR},${y + 1 + MARGEM_DO_QR}`)).toBe(false);
      expect(escuros.has(`${x + 3 + MARGEM_DO_QR},${y + 3 + MARGEM_DO_QR}`)).toBe(true);
    }
    for (const ponto of escuros) {
      const [cx, cy] = ponto.split(',').map(Number);
      expect(cx >= MARGEM_DO_QR && cy >= MARGEM_DO_QR && cx < lado - MARGEM_DO_QR && cy < lado - MARGEM_DO_QR).toBe(true);
    }
    expect(desenharQr('https://10.0.0.20:5443').caminho).not.toBe(caminho);
  });

  it('ligar e desligar aparecem em português nos acessos recentes', () => {
    expect(descreverEvento('AcessoCelularLigado')).toBe('Acesso pelo celular ligado');
    expect(descreverEvento('AcessoCelularDesligado')).toBe('Acesso pelo celular desligado');
  });
});
