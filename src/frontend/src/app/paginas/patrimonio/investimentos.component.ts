import { DatePipe, DecimalPipe } from '@angular/common';
import { Component, OnInit, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { ConfirmacaoService } from '../../compartilhado/ui/confirmacao.service';
import { FormsModule } from '@angular/forms';
import { CampoValorComponent } from '../../compartilhado/ui/campo-valor.component';
import { CampoDataComponent } from '../../compartilhado/ui/campo-data.component';
import { ActivatedRoute } from '@angular/router';
import { ApiService } from '../../core/api.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import {
  AtualizarAtivo, Carteira, InvestimentoComparado, MoedaCambio, OperacaoAtivo, PosicaoAtivo, Categoria, Reserva, SaidaReserva,
  SalvarOperacao, TipoOperacao
} from '../../core/modelos';
import { PrivacidadeService } from '../../core/privacidade.service';
import { AvisosService } from '../../core/avisos.service';
import { ModalComponent } from '../../compartilhado/ui/modal.component';
import { dataIso } from '../../compartilhado/utilidades';
import { MoedaPipe } from '../../compartilhado/moeda.pipe';
import { MaisAnalisesComponent } from '../../compartilhado/graficos/mais-analises.component';
import { InvestimentosFiisComponent } from './investimentos-fiis.component';
import { InvestimentosCambioComponent } from './investimentos-cambio.component';
import { InvestimentosCriptoComponent } from './investimentos-cripto.component';
import { ConferirInvestimentosComponent } from './conferir-investimentos.component';
import { RendimentoInvestimentosOfComponent } from './rendimento-investimentos-of.component';
import { conferenciaForaDoEsperado, diferencaComoGasto, valorEsperadoDaReserva } from './reserva-conferencia';
import { SaidaReservaComponent } from './saida-reserva.component';
import { ListaSuspensaDirective } from '../../compartilhado/ui/lista-suspensa.directive';
import { InvestimentosIndicadoresComponent } from './investimentos-indicadores.component';
import { InvestimentosEvolucaoComponent } from './investimentos-evolucao.component';
import { InvestimentosAnalisesComponent } from './investimentos-analises.component';
import { InvestimentosPosicoesComponent } from './investimentos-posicoes.component';
import { InvestimentosOpenFinanceComponent } from './investimentos-open-finance.component';
import { InvestimentosReservasComponent } from './investimentos-reservas.component';
import { InvestimentosOperacoesComponent } from './investimentos-operacoes.component';
import { InvestimentosAtivoComponent } from './investimentos-ativo.component';
import { InvestimentosImportacaoComponent } from './investimentos-importacao.component';
import {
  AbaInvestimentos, FormConferencia, LinhaImportacao, MensagemAviso, PtaxSugerida, abaDaUrl, abreConferirNaUrl, ativoParaEnviar,
  avisoDeLinhasIncompletas, contarReservasAConferir, dadosDaCompraQueFaltou, formularioConferencia, formularioDaOperacao,
  formularioNovaOperacao, itensDaImportacao, linhasDaImportacao, linhasParaImportar, mensagemCotacoes,
  mensagemImportacaoOpenFinance, nomeTipo, operacaoParaEnviar, perguntaDeConferenciaForaDoEsperado, posicoesAbertas,
  posicoesParaMostrar, ptaxDaOperacao, quantidadeDeFiis, saldoInformadoEmReais, tiposPara, totalOperacao
} from './investimentos';

/**
 * Investimentos (#731, ADR 0059): a tela guarda o estado e fala com a API; as partes que só desenham são os
 * `investimentos-*.component.ts` (indicadores, posições, Open Finance, reservas, gráficos e as janelas de operações,
 * ativo e importação), e a regra está em `investimentos.ts`. O formulário da operação, a conferência do saldo, o nome do
 * investimento e a fonte das cotações ficam aqui: editam o estado da tela direto (a PTAX que chega da API preenche o
 * câmbio do formulário; a conferência pede confirmação e fala com a API), e separá-los mexeria no estado mais central
 * da tela para um ganho pequeno.
 */
@Component({
  selector: 'app-investimentos',
  imports: [ListaSuspensaDirective, FormsModule, MoedaPipe, DatePipe, DecimalPipe, ModalComponent, MaisAnalisesComponent,
    InvestimentosFiisComponent, ConferirInvestimentosComponent, RendimentoInvestimentosOfComponent, CampoValorComponent, CampoDataComponent,
    InvestimentosCambioComponent, InvestimentosCriptoComponent, SaidaReservaComponent, InvestimentosIndicadoresComponent,
    InvestimentosEvolucaoComponent, InvestimentosAnalisesComponent, InvestimentosPosicoesComponent, InvestimentosOpenFinanceComponent,
    InvestimentosReservasComponent, InvestimentosOperacoesComponent, InvestimentosAtivoComponent, InvestimentosImportacaoComponent],
  templateUrl: './investimentos.component.html',
  styleUrl: './investimentos.component.css',
  changeDetection: ChangeDetectionStrategy.Eager
})
export class InvestimentosComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly confirmacao = inject(ConfirmacaoService);
  private readonly avisos = inject(AvisosService);
  private readonly privacidade = inject(PrivacidadeService);

  readonly TipoOperacao = TipoOperacao;
  readonly hoje = dataIso();
  readonly oculto = this.privacidade.oculto;
  readonly totalOperacao = totalOperacao;
  readonly tiposPara = tiposPara;
  readonly nomeTipo = nomeTipo;

  readonly dados = signal<Carteira | null>(null);
  private readonly rota = inject(ActivatedRoute);
  readonly aba = signal<AbaInvestimentos>('resumo');

  /** #331: a grade do Open Finance recolhida, lembrada neste navegador (ADR 0017). */
  static readonly CHAVE_GRADE_OF = 'financas.investimentos.gradeOFRecolhida';
  readonly gradeOfRecolhida = signal(InvestimentosComponent.lerGradeOf());

  alternarGradeOf(): void {
    this.gradeOfRecolhida.update((v) => !v);
    try {
      localStorage.setItem(InvestimentosComponent.CHAVE_GRADE_OF, this.gradeOfRecolhida() ? '1' : '0');
    } catch { /* sem armazenamento (modo privado): a grade só não lembra */ }
  }

  private static lerGradeOf(): boolean {
    try {
      return localStorage.getItem(InvestimentosComponent.CHAVE_GRADE_OF) === '1';
    } catch {
      return false;
    }
  }

  readonly mostrarEncerradas = signal(false);
  readonly atualizando = signal(false);
  readonly salvando = signal(false);
  readonly ativoAberto = signal<PosicaoAtivo | null>(null);
  readonly operacoes = signal<OperacaoAtivo[]>([]);
  readonly configAberta = signal(false);
  readonly versaoCarteira = signal(0);
  readonly importacaoAberta = signal(false);
  readonly conferenciaAberta = signal(false);
  readonly carregandoSugestoes = signal(false);
  readonly linhasImportacao = signal<LinhaImportacao[]>([]);
  readonly reservasAConferir = computed(() => contarReservasAConferir(this.dados()?.reservas));
  readonly quantidadeFiis = computed(() => quantidadeDeFiis(this.dados()?.posicoes));

  formOp: SalvarOperacao | null = null;
  editandoOpId: number | null = null;
  readonly moedas = signal<MoedaCambio[]>([]);
  readonly buscandoCambio = signal(false);
  /** Ativo já lançado: a moeda vem dele e não pode ser trocada. */
  moedaTravada = false;
  /** O usuário digitou o câmbio: mudar data/tipo não sobrescreve. */
  cambioEditado = false;
  ptaxSugerida: PtaxSugerida | null = null;
  /** O ativo da janela "Ativo · TICKER"; o formulário é do componente da janela, que edita uma cópia. */
  readonly ativoEditado = signal<PosicaoAtivo | null>(null);
  formConferencia: FormConferencia | null = null;
  formNome: { investimento: InvestimentoComparado; nome: string } | null = null;
  private atualizouAutomatico = false;
  private importouAutomatico = false;
  readonly importandoOpenFinance = signal(false);

  readonly abertas = computed(() => posicoesAbertas(this.dados()?.posicoes));
  readonly posicoesVisiveis = computed(() => posicoesParaMostrar(this.dados()?.posicoes, this.mostrarEncerradas()));

  ngOnInit(): void {
    this.aba.set(abaDaUrl(this.rota.snapshot.queryParamMap));
    if (abreConferirNaUrl(this.rota.snapshot.queryParamMap)) this.conferenciaAberta.set(true);
    this.carregar();
  }

  carregar(): void {
    this.api.carteira().subscribe({
      next: (c) => {
        this.dados.set(c);
        this.versaoCarteira.update((v) => v + 1);
        // Cotações antigas: atualiza uma vez ao abrir a página.
        // Investimentos do Open Finance ainda fora da carteira: registra as compras automaticamente (uma vez).
        if (c.openFinanceNaoRegistrados > 0 && !this.importouAutomatico) {
          this.importouAutomatico = true;
          this.importarOpenFinance(true);
          return;
        }
        if (c.cotacoesDesatualizadas && !this.atualizouAutomatico) {
          this.atualizouAutomatico = true;
          this.atualizarCotacoes(true);
        }
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  private avisar(m: MensagemAviso | null): void {
    if (!m) return;
    if (m.tipo === 'erro') this.avisos.erro(m.texto);
    else this.avisos.sucesso(m.texto);
  }

  atualizarCotacoes(silencioso = false): void {
    if (this.atualizando()) return;
    this.atualizando.set(true);
    this.api.atualizarCotacoes().subscribe({
      next: (r) => {
        this.atualizando.set(false);
        this.avisar(mensagemCotacoes(r, silencioso));
        this.carregar();
      },
      error: (e) => {
        this.atualizando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /* ------------------------------ operações ------------------------------ */

  novaOperacao(ticker = ''): void {
    const existente = ticker ? this.dados()?.posicoes.find((p) => p.ticker === ticker) : undefined;
    this.editandoOpId = null;
    this.moedaTravada = !!existente;
    this.cambioEditado = false;
    this.ptaxSugerida = null;
    this.formOp = formularioNovaOperacao(ticker, existente?.moeda, this.hoje);
    this.carregarMoedas();
    this.sugerirCambio();
  }

  editarOperacao(o: OperacaoAtivo): void {
    this.editandoOpId = o.id;
    this.moedaTravada = true;
    // Mantém o câmbio gravado; a PTAX aparece só como referência.
    this.cambioEditado = true;
    this.ptaxSugerida = null;
    this.formOp = formularioDaOperacao(o);
    this.sugerirCambio();
  }

  aoMudarMoeda(): void {
    if (!this.formOp) return;
    this.formOp.tipoAtivo = null;
    this.formOp.cambio = null;
    this.cambioEditado = false;
    this.sugerirCambio();
  }

  aoMudarDataOuTipo(): void {
    this.sugerirCambio();
  }

  usarPtax(): void {
    if (!this.formOp || !this.ptaxSugerida) return;
    this.formOp.cambio = this.ptaxSugerida.valor;
    this.cambioEditado = false;
  }

  /** Busca a PTAX da data (venda na compra, compra na venda) e preenche o câmbio, se o usuário não o digitou. */
  private sugerirCambio(): void {
    const f = this.formOp;
    this.ptaxSugerida = null;
    if (!f || f.moeda === 'BRL' || !f.data) return;
    const { moeda, data, tipo } = f;
    this.buscandoCambio.set(true);
    this.api.cotacaoCambio(moeda, data).subscribe({
      next: (c) => {
        this.buscandoCambio.set(false);
        // Ignora respostas atrasadas de uma escolha anterior.
        if (this.formOp !== f || f.moeda !== moeda || f.data !== data || f.tipo !== tipo) return;
        this.ptaxSugerida = ptaxDaOperacao(c, tipo);
        if (!this.cambioEditado) f.cambio = this.ptaxSugerida.valor;
      },
      error: () => this.buscandoCambio.set(false)
    });
  }

  private carregarMoedas(): void {
    if (this.moedas().length > 0) return;
    this.api.moedasCambio().subscribe({ next: (m) => this.moedas.set(m), error: () => this.moedas.set([]) });
  }

  salvarOperacao(): void {
    const f = this.formOp;
    if (!f) return;
    const dto = operacaoParaEnviar(f);
    this.salvando.set(true);
    const id = this.editandoOpId;
    const concluir = {
      next: () => {
        this.salvando.set(false);
        this.formOp = null;
        this.avisos.sucesso('Operação salva.');
        this.recarregarDepoisDeAlterar();
      },
      error: (e: unknown) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    };
    if (id) this.api.atualizarOperacao(id, dto).subscribe(concluir);
    else this.api.criarOperacao(dto).subscribe(concluir);
  }

  async excluirOperacao(): Promise<void> {
    const id = this.editandoOpId;
    if (!id || !(await this.confirmacao.confirmar({ titulo: 'Excluir operação', mensagem: 'Excluir esta operação?', confirmar: 'Excluir', perigo: true }))) return;
    this.api.excluirOperacao(id).subscribe({
      next: () => {
        this.formOp = null;
        this.avisos.sucesso('Operação excluída.');
        this.recarregarDepoisDeAlterar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  abrirOperacoes(p: PosicaoAtivo): void {
    this.operacoes.set([]);
    this.ativoAberto.set(p);
    this.api.operacoesAtivo(p.id).subscribe({
      next: (o) => this.operacoes.set(o),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /* ------------------------------ conferir investimentos ------------------------------ */

  /** Sem débito no extrato que explique a diferença: abre "Lançar compra" com o ativo e as cotas que faltam. */
  lancarCompraQueFaltou(e: { ticker: string; quantidade: number }): void {
    this.conferenciaAberta.set(false);
    this.novaOperacao(e.ticker);
    if (this.formOp) Object.assign(this.formOp, dadosDaCompraQueFaltou(e.quantidade));
  }

  verOperacoesDe(ticker: string): void {
    const posicao = this.dados()?.posicoes.find((p) => p.ticker === ticker);
    if (!posicao) return;
    this.conferenciaAberta.set(false);
    this.abrirOperacoes(posicao);
  }

  recarregarDepoisDeAlterar(): void {
    this.api.carteira().subscribe({
      next: (c) => {
        this.dados.set(c);
        this.versaoCarteira.update((v) => v + 1);
        const aberto = this.ativoAberto();
        if (aberto) {
          const atualizado = c.posicoes.find((p) => p.id === aberto.id);
          if (atualizado) this.abrirOperacoes(atualizado);
          else this.ativoAberto.set(null);
        }
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /* ------------------------------ ativo ------------------------------ */

  editarAtivo(p: PosicaoAtivo): void {
    this.ativoEditado.set(p);
  }

  salvarAtivo(form: AtualizarAtivo): void {
    const p = this.ativoEditado();
    if (!p) return;
    this.api.atualizarAtivo(p.id, ativoParaEnviar(form)).subscribe({
      next: () => {
        this.ativoEditado.set(null);
        this.avisos.sucesso('Ativo atualizado.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  async excluirAtivo(): Promise<void> {
    const p = this.ativoEditado();
    if (!p || !(await this.confirmacao.confirmar({ titulo: 'Excluir ativo', mensagem: `Excluir ${p.ticker} com todas as operações?`, confirmar: 'Excluir', perigo: true }))) return;
    this.api.excluirAtivo(p.id).subscribe({
      next: () => {
        this.ativoEditado.set(null);
        this.avisos.sucesso('Ativo excluído.');
        this.carregar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /* ------------------------------ nome (#170) ------------------------------ */

  abrirRenomear(i: InvestimentoComparado): void {
    this.formNome = { investimento: i, nome: i.nomePersonalizado ?? '' };
  }

  salvarNome(): void {
    const f = this.formNome;
    if (!f) return;
    const nome = f.nome.trim() || null;
    this.salvando.set(true);
    this.api.renomearInvestimento(f.investimento.id, nome).subscribe({
      next: () => {
        this.salvando.set(false);
        this.formNome = null;
        this.avisos.sucesso(nome ? 'Investimento renomeado.' : 'O investimento voltou ao nome automático.');
        this.carregar();
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /* ------------------------------ reservas ------------------------------ */

  abrirConferencia(r: Reserva): void {
    this.formConferencia = formularioConferencia(r, this.hoje);
  }

  /** #691: a diferença que pode virar gasto do cofrinho (0 = não oferece; reserva-conferencia.ts). */
  diferencaComoGasto(f: FormConferencia): number {
    return diferencaComoGasto(f.reserva, f.saldo, f.data, this.hoje);
  }

  /** #691: a reserva com o formulário de saída aberto, e as categorias dele (lidas na primeira abertura). */
  readonly saidaDe = signal<Reserva | null>(null);
  readonly categorias = signal<Categoria[]>([]);

  abrirSaida(r: Reserva): void {
    if (!this.categorias().length) this.api.categorias().subscribe({ next: (c) => this.categorias.set(c), error: () => this.categorias.set([]) });
    this.saidaDe.set(r);
  }

  registrarSaida(dto: SaidaReserva): void {
    this.salvando.set(true);
    this.api.registrarSaidaReserva(dto).subscribe({
      next: () => {
        this.salvando.set(false);
        this.saidaDe.set(null);
        this.avisos.sucesso(`Saída do cofrinho ${dto.nome} registrada.`);
        this.recarregarDepoisDeAlterar();
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  async salvarConferencia(): Promise<void> {
    const f = this.formConferencia;
    if (!f || f.saldo === null) return;
    const emDolar = f.reserva.moeda === 'USD';
    // A comparação com o esperado é em reais; em dólar sem cotação conhecida, não dá para comparar.
    const emReais = saldoInformadoEmReais(f.reserva, f.moeda, f.saldo);
    if (emReais !== null && !(await this.conferenciaPlausivel(f.reserva, emReais))) return;
    this.salvando.set(true);
    const diferenca = f.registrarDiferenca && this.diferencaComoGasto(f) > 0;
    this.api.conferirReserva(f.reserva.contaId, f.reserva.nome, f.saldo, f.data, emDolar ? f.moeda : undefined, diferenca).subscribe({
      next: () => {
        this.salvando.set(false);
        this.formConferencia = null;
        this.avisos.sucesso(diferenca ? 'Saldo conferido, e a diferença registrada como gasto do cofrinho.' : 'Saldo da reserva conferido.');
        this.recarregarDepoisDeAlterar();
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /** #168: valor muito diferente do esperado pede confirmação antes de salvar (reserva-conferencia.ts). */
  async conferenciaPlausivel(r: Reserva, saldo: number): Promise<boolean> {
    if (!conferenciaForaDoEsperado(r, saldo)) return true;
    return this.confirmacao.confirmar({
      titulo: 'Conferir o saldo',
      mensagem: perguntaDeConferenciaForaDoEsperado(r, saldo, valorEsperadoDaReserva(r)),
      confirmar: 'Salvar assim mesmo'
    });
  }

  async removerConferencia(): Promise<void> {
    const f = this.formConferencia;
    if (!f || !(await this.confirmacao.confirmar({ titulo: 'Apagar conferência', mensagem: `Apagar a conferência de ${f.reserva.nome}? O saldo volta a ser estimado só pelos lançamentos.`, confirmar: 'Apagar', perigo: true }))) return;
    this.api.removerConferenciaReserva(f.reserva.contaId, f.reserva.nome).subscribe({
      next: () => {
        this.formConferencia = null;
        this.avisos.sucesso('Conferência apagada.');
        this.recarregarDepoisDeAlterar();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  /* ------------------------------ importação do Open Finance ------------------------------ */

  importarOpenFinance(automatico = false): void {
    if (this.importandoOpenFinance()) return;
    this.importandoOpenFinance.set(true);
    this.importouAutomatico = true;
    this.atualizouAutomatico = true; // a importação já atualiza as cotações
    this.api.importarOpenFinance().subscribe({
      next: (r) => {
        this.importandoOpenFinance.set(false);
        this.avisar(mensagemImportacaoOpenFinance(r, automatico));
        this.recarregarDepoisDeAlterar();
      },
      error: (e) => {
        this.importandoOpenFinance.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /* ------------------------------ importação pelo extrato ------------------------------ */

  abrirImportacao(): void {
    this.importacaoAberta.set(true);
    this.carregandoSugestoes.set(true);
    this.linhasImportacao.set([]);
    this.api.sugestoesExtrato().subscribe({
      next: (s) => {
        this.carregandoSugestoes.set(false);
        this.linhasImportacao.set(linhasDaImportacao(s));
      },
      error: (e) => {
        this.carregandoSugestoes.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /** As linhas vêm da janela, com o que a pessoa marcou e corrigiu: aqui se confere e se importa. */
  importarSelecionadas(linhas: LinhaImportacao[]): void {
    const selecionadas = linhasParaImportar(linhas);
    const aviso = avisoDeLinhasIncompletas(selecionadas);
    if (aviso) {
      this.avisos.erro(aviso);
      return;
    }
    this.salvando.set(true);
    this.api.importarExtrato(itensDaImportacao(selecionadas)).subscribe({
      next: (r) => {
        this.salvando.set(false);
        this.importacaoAberta.set(false);
        this.avisos.sucesso(`${r.transacoesAtualizadas} operação(ões) importada(s).`);
        this.recarregarDepoisDeAlterar();
      },
      error: (e) => {
        this.salvando.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  /* ------------------------------ configuração ------------------------------ */

  abrirConfiguracao(): void {
    this.configAberta.set(true);
  }
}
