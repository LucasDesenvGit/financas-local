import { ChangeDetectionStrategy, Component, OnInit, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ApiService } from '../../core/api.service';
import { AvisosService } from '../../core/avisos.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import { SituacaoCelular } from '../../core/modelos';
import { ConfirmacaoService } from '../../compartilhado/ui/confirmacao.service';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { CAMINHO_AUTORIDADE, abertoPelaRede, enderecoDaAutoridade, ipInicial } from './acesso-celular';

/**
 * #328 (ADR 0061): o acesso pelo celular na rede de casa, em Configurações › Segurança. Desligado por padrão. Ligar
 * pede confirmação, porque abre a tela de login para quem está na mesma rede (Nielsen 5, prevenção de erro); desligar
 * é na hora, menos quando a própria página veio pela rede (ela para de responder).
 */
@Component({
  selector: 'app-acesso-celular',
  imports: [ListaSuspensaDirective, FormsModule],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <section class="painel-bloco acesso-celular" aria-labelledby="titulo-acesso-celular">
      <h2 id="titulo-acesso-celular">Acesso pelo celular</h2>
      @if (situacao(); as s) {
        @if (s.ligado) {
          <p class="estado-celular" role="status">
            <strong>Ligado.</strong> No celular, no mesmo Wi-Fi, abra <strong class="endereco-celular">{{ s.endereco }}</strong>
          </p>
          @for (a of s.avisos; track a) { <p class="aviso-inline">{{ a }}</p> }
          @if (s.ipIndisponivel) {
            <p class="aviso-inline">O endereço {{ s.ip }} não está mais neste computador (o roteador pode ter dado outro).
              Desligue e ligue de novo escolhendo o endereço novo.</p>
          }
          <details class="guia" open>
            <summary>Como abrir no celular (uma vez)</summary>
            <ol>
              <li>No celular, conectado ao mesmo Wi-Fi, abra <strong class="quebra">{{ autoridade() }}</strong>. Desta vez o navegador
                avisa que a conexão não é particular: é esperado, porque o celular ainda não conhece o certificado. Siga em
                “Avançado” e “Continuar”.</li>
              <li><strong>Android:</strong> abra o arquivo baixado ou vá em Configurações › Segurança › Criptografia e credenciais ›
                Instalar um certificado › Certificado de CA, e escolha “{{ nomeDoArquivo }}”.</li>
              <li><strong>iPhone:</strong> em Ajustes › Perfil Transferido, toque em Instalar. Depois, em Ajustes › Geral › Sobre ›
                Ajustes de Confiança de Certificados, ligue “Finanças Local (celular)”.</li>
              <li>Abra <strong class="quebra">{{ s.endereco }}</strong> e entre com o seu usuário e a sua senha.</li>
            </ol>
            <p class="texto-suave">Para conferir que o certificado é deste computador, compare a impressão digital (SHA-256), que o
              iPhone mostra em “Mais detalhes” ao instalar:</p>
            <p><code class="impressao-digital">{{ s.impressaoDigital }}</code></p>
            <p class="texto-suave">Prefere mandar o arquivo de outro jeito (cabo, e-mail para você mesmo)?
              <a [href]="caminhoAutoridade" download="financas-local-celular.crt">Baixar o certificado neste computador</a>.</p>
          </details>
          <div class="rodape-form"><span class="cresce"></span>
            <button type="button" class="botao" (click)="desligar()" [disabled]="ocupado()">Desligar o acesso pelo celular</button>
          </div>
        } @else {
          <p>Abra o site no celular, na mesma rede Wi-Fi de casa. Desligado, o site só abre neste computador.</p>
          <p class="aviso-inline">Ligado, qualquer aparelho da sua rede chega à tela de entrar: use uma senha forte e não ligue em
            rede pública (café, aeroporto). Fora de casa, o celular não chega ao site.</p>
          @if (s.ipIndisponivel && s.ip) {
            <p class="aviso-inline">O acesso estava ligado em {{ s.ip }}, que não está mais neste computador (o roteador pode ter
              dado outro). Escolha o endereço de novo.</p>
          }
          @if (s.enderecos.length) {
            <div class="formulario">
              <label>Endereço deste computador na rede
                <select name="ipCelular" [(ngModel)]="ipEscolhido" aria-describedby="ajuda-ip-celular">
                  @for (e of s.enderecos; track e.ip) { <option [value]="e.ip">{{ e.ip }} ({{ e.interface }})</option> }
                </select>
              </label>
              <small id="ajuda-ip-celular" class="texto-suave">Com mais de um, escolha o do Wi-Fi ou do cabo da sua casa. Para o
                endereço não mudar, reserve esse IP no roteador.</small>
              <div class="rodape-form"><span class="cresce"></span>
                <button type="button" class="botao botao-primario" (click)="ligar()" [disabled]="ocupado() || !ipEscolhido">
                  Ligar o acesso pelo celular</button>
              </div>
            </div>
          } @else {
            <p class="vazio">Este computador não está numa rede de casa agora (Wi-Fi ou cabo). Conecte e abra esta tela de novo.</p>
          }
        }
      } @else if (erro()) {
        <p class="vazio">{{ erro() }}</p>
      } @else {
        <p class="vazio">Carregando…</p>
      }
    </section>
  `,
  styles: [`
    .acesso-celular .endereco-celular, .acesso-celular .quebra, .impressao-digital { overflow-wrap: anywhere; }
    .impressao-digital { font-size: 0.85rem; }
    .acesso-celular ol { padding-left: 1.2rem; display: grid; gap: 6px; }
  `]
})
export class AcessoCelularComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  private readonly confirmacao = inject(ConfirmacaoService);

  readonly situacao = signal<SituacaoCelular | null>(null);
  readonly erro = signal<string | null>(null);
  readonly ocupado = signal(false);
  readonly caminhoAutoridade = CAMINHO_AUTORIDADE;
  readonly nomeDoArquivo = 'financas-local-celular.crt';
  ipEscolhido = '';

  ngOnInit(): void {
    this.api.situacaoCelular().subscribe({
      next: (s) => this.mostrar(s),
      error: (e) => this.erro.set(mensagemDeErro(e))
    });
  }

  autoridade(): string | null {
    const s = this.situacao();
    return s ? enderecoDaAutoridade(s) : null;
  }

  async ligar(): Promise<void> {
    const ip = this.ipEscolhido;
    const ok = await this.confirmacao.confirmar({
      titulo: 'Ligar o acesso pelo celular',
      mensagem: `O site passa a abrir também em ${ip}, para os aparelhos da sua rede. Quem estiver nela chega à tela de entrar, ` +
        'então a sua senha passa a ser a proteção. Ligar?',
      confirmar: 'Ligar'
    });
    if (!ok) return;
    this.ocupado.set(true);
    this.api.ligarCelular(ip).subscribe({
      next: (s) => {
        this.mostrar(s);
        this.ocupado.set(false);
        this.avisos.sucesso('Acesso pelo celular ligado.');
      },
      error: (e) => {
        this.ocupado.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  async desligar(): Promise<void> {
    if (abertoPelaRede(location.hostname)) {
      const ok = await this.confirmacao.confirmar({
        titulo: 'Desligar o acesso pelo celular',
        mensagem: 'Você abriu esta página pela rede: depois de desligar, ela para de responder neste aparelho. Desligar?',
        confirmar: 'Desligar',
        perigo: true
      });
      if (!ok) return;
    }
    this.ocupado.set(true);
    this.api.desligarCelular().subscribe({
      next: (s) => {
        this.mostrar(s);
        this.ocupado.set(false);
        this.avisos.sucesso('Acesso pelo celular desligado. O site só abre neste computador.');
      },
      error: (e) => {
        this.ocupado.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  private mostrar(s: SituacaoCelular): void {
    this.situacao.set(s);
    this.erro.set(null);
    this.ipEscolhido = ipInicial(s);
  }
}
