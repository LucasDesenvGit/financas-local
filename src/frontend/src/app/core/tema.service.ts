import { Injectable, computed, signal } from '@angular/core';

export type PreferenciaTema = 'claro' | 'escuro' | 'sistema';
export type Tema = 'claro' | 'escuro';
export type Paleta = 'lavanda' | 'oceano' | 'floresta' | 'grafite' | 'por-do-sol' | 'personalizada';

export interface OpcaoPaleta { id: Paleta; nome: string; amostras: string[]; }

/** Paletas prontas (as cores ficam no styles.css; as amostras aqui são só para a tela de escolha). */
export const PALETAS: OpcaoPaleta[] = [
  { id: 'lavanda', nome: 'Lavanda', amostras: ['#6267da', '#b9b6ff', '#9fd2f6'] },
  { id: 'oceano', nome: 'Oceano', amostras: ['#2a78b8', '#9fd2f6', '#8ee6d6'] },
  { id: 'floresta', nome: 'Floresta', amostras: ['#26845b', '#9fe0bf', '#cfe8a0'] },
  { id: 'grafite', nome: 'Grafite', amostras: ['#4b5563', '#cfd4db', '#aeb6c2'] },
  { id: 'por-do-sol', nome: 'Pôr do sol', amostras: ['#c05339', '#ffb38f', '#f7a8c4'] }
];

const COR_PADRAO = '#6267da';
/** Variáveis que a paleta personalizada define inline (precisam ser limpas ao trocar de paleta). */
const VARIAVEIS_PERSONALIZADAS = ['--fundo', '--superficie', '--superficie-2', '--superficie-3', '--linha', '--primaria', '--primaria-hover', '--primaria-texto', '--primaria-suave',
  '--primaria-forte', '--destaque-fundo', '--destaque-texto', '--destaque-suave', '--marca-fundo', '--marca-texto'];

/**
 * Tema claro/escuro e paleta de cores. As preferências ficam só neste navegador (localStorage);
 * "sistema" acompanha a configuração do sistema operacional.
 */
@Injectable({ providedIn: 'root' })
export class TemaService {
  private static readonly CHAVE = 'financas.tema';
  private static readonly CHAVE_PALETA = 'financas.paleta';
  private static readonly CHAVE_COR = 'financas.cor';
  private readonly midia = window.matchMedia('(prefers-color-scheme: dark)');

  readonly preferencia = signal<PreferenciaTema>(this.lerPreferencia());
  /** Tema efetivamente aplicado. */
  readonly tema = signal<Tema>('claro');
  readonly paleta = signal<Paleta>(this.lerPaleta());
  readonly corPersonalizada = signal<string>(this.lerCor());
  /** Muda sempre que alguma cor da tela muda (os gráficos observam este sinal para redesenhar). */
  readonly aparencia = computed(() => `${this.tema()}|${this.paleta()}|${this.corPersonalizada()}`);

  constructor() {
    this.aplicar();
    this.midia.addEventListener('change', () => {
      if (this.preferencia() === 'sistema') this.aplicar();
    });
  }

  definir(preferencia: PreferenciaTema): void {
    this.preferencia.set(preferencia);
    this.gravar(TemaService.CHAVE, preferencia);
    this.aplicar();
  }

  alternar(): void {
    this.definir(this.tema() === 'escuro' ? 'claro' : 'escuro');
  }

  definirPaleta(paleta: Paleta): void {
    this.paleta.set(paleta);
    this.gravar(TemaService.CHAVE_PALETA, paleta);
    this.aplicar();
  }

  definirCorPersonalizada(cor: string): void {
    if (!/^#[0-9a-f]{6}$/i.test(cor)) return;
    this.corPersonalizada.set(cor.toLowerCase());
    this.gravar(TemaService.CHAVE_COR, cor.toLowerCase());
    this.definirPaleta('personalizada');
  }

  private aplicar(): void {
    const pref = this.preferencia();
    const tema: Tema = pref === 'sistema' ? (this.midia.matches ? 'escuro' : 'claro') : pref;
    const raiz = document.documentElement;
    raiz.dataset['tema'] = tema;
    raiz.dataset['paleta'] = this.paleta();
    for (const v of VARIAVEIS_PERSONALIZADAS) raiz.style.removeProperty(v);
    if (this.paleta() === 'personalizada') {
      for (const [v, valor] of Object.entries(variaveisDaCor(this.corPersonalizada(), tema))) raiz.style.setProperty(v, valor);
    }
    this.tema.set(tema);
  }

  private gravar(chave: string, valor: string): void {
    try {
      localStorage.setItem(chave, valor);
    } catch {
      /* armazenamento indisponível: vale só nesta sessão */
    }
  }

  private ler(chave: string): string | null {
    try {
      return localStorage.getItem(chave);
    } catch {
      return null;
    }
  }

  private lerPreferencia(): PreferenciaTema {
    const valor = this.ler(TemaService.CHAVE);
    return valor === 'claro' || valor === 'escuro' || valor === 'sistema' ? valor : 'sistema';
  }

  private lerPaleta(): Paleta {
    const valor = this.ler(TemaService.CHAVE_PALETA);
    return valor === 'personalizada' || PALETAS.some((p) => p.id === valor) ? (valor as Paleta) : 'lavanda';
  }

  private lerCor(): string {
    const valor = this.ler(TemaService.CHAVE_COR);
    return valor && /^#[0-9a-f]{6}$/i.test(valor) ? valor : COR_PADRAO;
  }
}

/* ------------------------------------------------------------------ Paleta personalizada */

/**
 * Deriva as variáveis de cor a partir de uma única cor (matiz e saturação dela). No tema claro a cor principal é
 * escurecida até o texto branco ter contraste >= 4,5:1; no escuro, clareada até o texto escuro ter o mesmo contraste.
 */
export function variaveisDaCor(hex: string, tema: Tema): Record<string, string> {
  const [h, s] = paraHsl(hex);
  const sat = Math.max(s, 25);
  if (tema === 'claro') {
    let l = 50;
    while (l > 15 && contraste(hsl(h, sat, l), '#ffffff') < 4.5) l -= 1;
    return {
      '--fundo': hsl(h, 30, 97.5),
      '--superficie': '#ffffff',
      '--superficie-2': hsl(h, 30, 96),
      '--superficie-3': hsl(h, 28, 92.5),
      '--linha': hsl(h, 25, 92),
      '--primaria': hsl(h, sat, l),
      '--primaria-hover': hsl(h, sat, l - 5),
      '--primaria-texto': '#ffffff',
      '--primaria-suave': hsl(h, Math.min(sat, 80), 95),
      '--primaria-forte': hsl(h, sat, Math.max(l - 12, 15)),
      '--destaque-fundo': `linear-gradient(135deg, ${hsl(h, 70, 90)} 0%, ${hsl(h + 35, 60, 91)} 55%, ${hsl(h + 70, 50, 93)} 100%)`,
      '--destaque-texto': hsl(h, 35, 14),
      '--destaque-suave': hsl(h, 18, 36),
      '--marca-fundo': `linear-gradient(135deg, ${hsl(h, 75, 80)}, ${hsl(h + 35, 70, 82)})`,
      '--marca-texto': hsl(h, 45, 18)
    };
  }
  const texto = hsl(h, 40, 12);
  let l = 68;
  while (l < 92 && contraste(hsl(h, sat, l), texto) < 4.5) l += 1;
  return {
    '--fundo': hsl(h, 15, 8),
    '--superficie': hsl(h, 13, 11.5),
    '--superficie-2': hsl(h, 13, 14.5),
    '--superficie-3': hsl(h, 13, 18.5),
    '--linha': hsl(h, 12, 20),
    '--primaria': hsl(h, sat, l),
    '--primaria-hover': hsl(h, sat, Math.min(l + 5, 95)),
    '--primaria-texto': texto,
    '--primaria-suave': hsl(h, 30, 21),
    '--primaria-forte': hsl(h, sat, Math.min(l + 10, 95)),
    '--destaque-fundo': `linear-gradient(135deg, ${hsl(h, 35, 25)} 0%, ${hsl(h + 35, 30, 22)} 55%, ${hsl(h + 70, 28, 20)} 100%)`,
    '--destaque-texto': hsl(h, 40, 96),
    '--destaque-suave': hsl(h, 20, 78),
    '--marca-fundo': `linear-gradient(135deg, ${hsl(h, 75, 80)}, ${hsl(h + 35, 70, 82)})`,
    '--marca-texto': hsl(h, 45, 18)
  };
}

function paraHsl(hex: string): [number, number, number] {
  const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255);
  const max = Math.max(r, g, b), min = Math.min(r, g, b);
  const l = (max + min) / 2;
  if (max === min) return [0, 0, l * 100];
  const d = max - min;
  const s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
  const h = max === r ? (g - b) / d + (g < b ? 6 : 0) : max === g ? (b - r) / d + 2 : (r - g) / d + 4;
  return [h * 60, s * 100, l * 100];
}

/** Converte HSL para hex (#rrggbb), para poder medir contraste. */
function hsl(h: number, s: number, l: number): string {
  h = ((h % 360) + 360) % 360;
  s = Math.min(100, Math.max(0, s)) / 100;
  l = Math.min(100, Math.max(0, l)) / 100;
  const k = (n: number) => (n + h / 30) % 12;
  const a = s * Math.min(l, 1 - l);
  const f = (n: number) => l - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)));
  return '#' + [f(0), f(8), f(4)].map((x) => Math.round(x * 255).toString(16).padStart(2, '0')).join('');
}

function luminancia(hex: string): number {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contraste(a: string, b: string): number {
  const [x, y] = [luminancia(a), luminancia(b)].sort((p, q) => q - p);
  return (x + 0.05) / (y + 0.05);
}
