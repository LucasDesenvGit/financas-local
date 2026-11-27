import { ChangeDetectionStrategy, Component, OnInit, computed, inject, signal } from '@angular/core';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { ApiService } from '../../core/api.service';
import { AvisosService } from '../../core/avisos.service';
import { mensagemDeErro } from '../../core/erro.interceptor';
import {
  Emprego, Holerite, LancamentoHolerite, OrigemHolerite, PreviaHoleritePdf, ResumoHolerite, SituacaoHolerite, TipoLancamentoHolerite
} from '../../core/modelos';
import { ConfirmacaoService } from '../../compartilhado/ui/confirmacao.service';
import { MesAno } from '../../compartilhado/ui/seletor-mes.component';
import { estadoDoFormulario, lancamentoNovo, paraSalvar, trocarTipo } from './holerites';
import { empregoAtual, mesDaApi, nomeDoMes } from './salario';
import { HoleritesDetalheComponent } from './holerites-detalhe.component';
import { HoleritesListaComponent } from './holerites-lista.component';
import { HoleritesSenhaComponent } from './holerites-senha.component';
import { HoleritePdfPreviaComponent } from './holerite-pdf-previa.component';
import { LinhaPrevia, linhasDaPrevia, mesDoPdf, motivoDaRecusa, paraBase64, paraConfirmar, problemaDoArquivo } from './holerite-pdf';

/**
 * #211 (ADR 0065): Trabalho › Holerites. A lista dos meses (bruto, descontos, líquido) e o holerite escolhido: os
 * lançamentos do mês (horas extras, adicional noturno, bônus, descontos), o cálculo linha a linha e "Fechar o mês", que
 * congela os números (Nielsen 3: "Reabrir" desfaz, e fica nos acessos recentes). O holerite aberto fica na URL
 * (?holerite=), para o voltar do navegador funcionar (ADR 0018). Estimativa, não holerite; não é receita.
 * #228 (ADR 0068): "Ler holerite em PDF" (um ou vários) mostra a prévia de cada um, e só grava com "Confirmar", com
 * "Desfazer" no aviso (Nielsen 3). O lido do PDF fica marcado na lista e no holerite (Nielsen 1).
 * #777 (ADR 0059): a tela guarda o estado e fala com a API; a lista, a senha do PDF e o holerite aberto são componentes
 * de apresentação (`holerites-*.component.ts`) e a regra está em `holerites.ts`.
 */
@Component({
  selector: 'app-trabalho-holerites',
  imports: [RouterLink, HoleritesListaComponent, HoleritesSenhaComponent, HoleritesDetalheComponent, HoleritePdfPreviaComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <header class="cabecalho-pagina"><h1>Holerites</h1></header>

    @if (carregando()) {
      <p class="texto-suave" role="status">Carregando…</p>
    } @else if (!emprego()) {
      <section class="painel-bloco">
        <p>Cadastre o emprego antes, em <a routerLink="/trabalho/salario">Trabalho › Salário</a>: o holerite parte do salário dele.</p>
      </section>
    } @else {
      <p class="texto-suave aviso-estimativa">Estimativa, não holerite: confira com o da empresa. Nada daqui entra nos relatórios.</p>
      <div class="grade-lateral">
        <app-holerites-lista [empregador]="emprego()!.empregador" [lista]="lista()" [abertoId]="aberto()?.id ?? null" [mesNovo]="mesNovo()"
          [ocupado]="ocupado()" (mesNovoMudou)="mesNovo.set($event)" (criar)="criar()" (abrir)="abrir($event)" (escolherPdfs)="escolherPdfs($event)" />

        @if (pedindoSenha()) {
          <app-holerites-senha [arquivo]="arquivoAtual()" [restantes]="fila().length" [ocupado]="ocupado()"
            (abrirComSenha)="lerPdf($event)" (pular)="pularPdf()" />
        } @else if (previa(); as p) {
          <app-holerite-pdf-previa [previa]="p" [linhas]="linhasPdf()" [mes]="mesPdf()" [arquivo]="arquivoAtual()"
            [restantes]="fila().length" [ocupado]="ocupado()" (linhasMudaram)="linhasPdf.set($event)" (mesMudou)="mudarMesPdf($event)"
            (confirmar)="confirmarPdf()" (cancelar)="pularPdf()" />
        } @else if (aberto(); as h) {
          <app-holerites-detalhe [holerite]="h" [lancamentos]="lancamentos()" [mudou]="mudou()" [ocupado]="ocupado()"
            [(diasUteis)]="diasUteis" [(diasDescanso)]="diasDescanso" (salvar)="salvar()" (fechar)="fechar()" (reabrir)="reabrir()"
            (excluir)="excluir()" (acrescentar)="acrescentar()" (remover)="remover($event)" (mudarTipo)="mudarTipo($event.indice, $event.tipo)" />
        }
      </div>
    }
  `,
  styles: `
    .grade-lateral { display: grid; grid-template-columns: minmax(240px, 1fr) minmax(0, 2.4fr); gap: 16px; align-items: start; }
    @media (max-width: 900px) { .grade-lateral { grid-template-columns: minmax(0, 1fr); } }
    .aviso-estimativa { margin-top: 0; }
  `
})
export class TrabalhoHoleritesComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly avisos = inject(AvisosService);
  private readonly confirmacao = inject(ConfirmacaoService);
  private readonly rota = inject(ActivatedRoute);
  private readonly router = inject(Router);

  readonly carregando = signal(true);
  readonly ocupado = signal(false);
  readonly emprego = signal<Emprego | null>(null);
  readonly lista = signal<ResumoHolerite[]>([]);
  readonly aberto = signal<Holerite | null>(null);
  readonly fechado = computed(() => this.aberto()?.situacao === SituacaoHolerite.Fechado);
  readonly lido = computed(() => this.aberto()?.origem === OrigemHolerite.LidoDoPdf);

  /** #228: o PDF em leitura (a prévia, a senha) e os que esperam na fila. O conteúdo fica só na memória da tela. */
  readonly previa = signal<PreviaHoleritePdf | null>(null);
  readonly linhasPdf = signal<LinhaPrevia[]>([]);
  readonly mesPdf = signal<string | null>(null);
  readonly pedindoSenha = signal(false);
  readonly arquivoAtual = signal('');
  readonly fila = signal<File[]>([]);
  private conteudoPdf = '';
  readonly lancamentos = signal<LancamentoHolerite[]>([]);
  readonly mesNovo = signal<MesAno>({ ano: new Date().getFullYear(), mes: new Date().getMonth() + 1 });
  readonly diasUteis = signal(0);
  readonly diasDescanso = signal(0);
  /** O texto do que está salvo, para saber se a pessoa mexeu (e pedir para salvar antes de fechar). */
  private salvo = '';

  ngOnInit(): void {
    this.api.empregos().subscribe({
      next: (lista) => {
        this.emprego.set(empregoAtual(lista));
        this.carregando.set(false);
        this.recarregarLista();
        const id = Number(this.rota.snapshot.queryParamMap.get('holerite'));
        if (id) this.abrir(id);
      },
      error: (e) => { this.carregando.set(false); this.avisos.erro(mensagemDeErro(e)); }
    });
  }

  /** Mexeu nos lançamentos ou nos dias desde o que está salvo. */
  mudou(): boolean {
    return this.estado() !== this.salvo;
  }

  private estado(): string {
    return estadoDoFormulario(this.diasUteis(), this.diasDescanso(), this.lancamentos());
  }

  private recarregarLista(): void {
    const e = this.emprego();
    if (!e) return;
    this.api.holerites(e.id).subscribe({ next: (l) => this.lista.set(l), error: (err) => this.avisos.erro(mensagemDeErro(err)) });
  }

  private mostrar(h: Holerite): void {
    this.aberto.set(h);
    this.diasUteis.set(h.diasUteis);
    this.diasDescanso.set(h.diasDescanso);
    this.lancamentos.set(h.lancamentos.map((l) => ({ ...l })));
    this.salvo = this.estado();
    this.router.navigate([], { relativeTo: this.rota, queryParams: { holerite: h.id }, replaceUrl: true });
  }

  abrir(id: number): void {
    this.api.holerite(id).subscribe({ next: (h) => this.mostrar(h), error: (e) => this.avisos.erro(mensagemDeErro(e)) });
  }

  criar(): void {
    const e = this.emprego();
    if (!e) return;
    this.ocupado.set(true);
    this.api.criarHolerite(e.id, mesDaApi(this.mesNovo())).subscribe({
      next: (h) => { this.ocupado.set(false); this.mostrar(h); this.recarregarLista(); },
      error: (err) => { this.ocupado.set(false); this.avisos.erro(mensagemDeErro(err)); }
    });
  }

  acrescentar(): void {
    this.lancamentos.update((l) => [...l, lancamentoNovo(TipoLancamentoHolerite.HoraExtra)]);
  }

  remover(i: number): void {
    this.lancamentos.update((l) => l.filter((_, j) => j !== i));
  }

  mudarTipo(i: number, tipo: TipoLancamentoHolerite): void {
    this.lancamentos.update((l) => l.map((x, j) => (j === i ? trocarTipo(x, Number(tipo)) : x)));
  }

  salvar(): void {
    const h = this.aberto();
    if (!h) return;
    this.ocupado.set(true);
    this.api.salvarHolerite(h.id, this.diasUteis(), this.diasDescanso(), paraSalvar(this.lancamentos())).subscribe({
      next: (novo) => { this.ocupado.set(false); this.mostrar(novo); this.recarregarLista(); this.avisos.sucesso('Holerite calculado.'); },
      error: (err) => { this.ocupado.set(false); this.avisos.erro(mensagemDeErro(err)); }
    });
  }

  fechar(): void {
    const h = this.aberto();
    if (!h) return;
    this.acao(this.api.fecharHolerite(h.id), `Holerite de ${nomeDoMes(h.mes)} fechado: os números ficam como estão.`);
  }

  reabrir(): void {
    const h = this.aberto();
    if (!h) return;
    this.acao(this.api.reabrirHolerite(h.id), 'Holerite reaberto: ele volta a ser calculado com o salário e as tabelas de agora.');
  }

  private acao(pedido: ReturnType<ApiService['fecharHolerite']>, mensagem: string): void {
    this.ocupado.set(true);
    pedido.subscribe({
      next: (novo) => { this.ocupado.set(false); this.mostrar(novo); this.recarregarLista(); this.avisos.sucesso(mensagem); },
      error: (err) => { this.ocupado.set(false); this.avisos.erro(mensagemDeErro(err)); }
    });
  }

  async excluir(): Promise<void> {
    const h = this.aberto();
    if (!h) return;
    const ok = await this.confirmacao.confirmar({ titulo: 'Excluir holerite',
      mensagem: `Excluir o holerite de ${nomeDoMes(h.mes)} e os lançamentos dele?`, confirmar: 'Excluir', perigo: true });
    if (!ok) return;
    this.api.excluirHolerite(h.id).subscribe({
      next: () => {
        this.aberto.set(null);
        this.router.navigate([], { relativeTo: this.rota, queryParams: { holerite: null }, replaceUrl: true });
        this.recarregarLista();
        this.avisos.sucesso('Holerite excluído.');
      },
      error: (err) => this.avisos.erro(mensagemDeErro(err))
    });
  }

  /** #228: os PDFs escolhidos entram na fila; cada um vira uma prévia, um de cada vez. */
  escolherPdfs(evento: Event): void {
    const campo = evento.target as HTMLInputElement;
    const arquivos = Array.from(campo.files ?? []);
    campo.value = '';
    if (!arquivos.length) return;
    this.fila.set(arquivos);
    this.proximoPdf();
  }

  private proximoPdf(): void {
    this.previa.set(null);
    this.pedindoSenha.set(false);
    this.conteudoPdf = '';
    const [arquivo, ...resto] = this.fila();
    this.fila.set(resto);
    this.arquivoAtual.set(arquivo?.name ?? '');
    if (!arquivo) return;
    const problema = problemaDoArquivo(arquivo.name, arquivo.size);
    if (problema) {
      this.avisos.erro(problema);
      this.proximoPdf();
      return;
    }
    this.ocupado.set(true);
    arquivo.arrayBuffer().then(
      (conteudo) => { this.conteudoPdf = paraBase64(new Uint8Array(conteudo)); this.lerPdf(''); },
      () => { this.ocupado.set(false); this.avisos.erro(`${arquivo.name}: não deu para abrir o arquivo.`); this.proximoPdf(); }
    );
  }

  lerPdf(senha: string): void {
    const e = this.emprego();
    if (!e) return;
    this.ocupado.set(true);
    this.api.lerHoleritePdf(e.id, this.conteudoPdf, senha).subscribe({
      next: (p) => {
        this.ocupado.set(false);
        this.pedindoSenha.set(false);
        this.previa.set(p);
        this.linhasPdf.set(linhasDaPrevia(p));
        this.mesPdf.set(mesDoPdf(p));
      },
      error: (err) => {
        this.ocupado.set(false);
        if (motivoDaRecusa(err) === 'senha') {
          if (senha) this.avisos.erro(mensagemDeErro(err));
          this.pedindoSenha.set(true);
          return;
        }
        this.avisos.erro(`${this.arquivoAtual()}: ${mensagemDeErro(err)}`);
        this.proximoPdf();
      }
    });
  }

  /** "Cancelar" ou "Pular este": nada é gravado, e vem o próximo da fila. */
  pularPdf(): void {
    this.proximoPdf();
  }

  mudarMesPdf(m: MesAno): void {
    this.mesPdf.set(`${mesDaApi(m)}-01`);
  }

  confirmarPdf(): void {
    const e = this.emprego();
    const p = this.previa();
    const mes = this.mesPdf();
    if (!e || !p || !mes) return;
    this.ocupado.set(true);
    this.api.confirmarHoleritePdf(e.id, paraConfirmar(p, this.linhasPdf(), mes)).subscribe({
      next: (g) => {
        this.ocupado.set(false);
        const nomeMes = nomeDoMes(g.holerite.mes);
        this.avisos.sucesso(`Holerite de ${nomeMes} gravado, lido do PDF.`,
          { rotulo: 'Desfazer', executar: () => this.desfazerPdf(e.id, g.desfazer, nomeMes) });
        this.recarregarLista();
        const ultimo = !this.fila().length;
        this.proximoPdf();
        if (ultimo) this.mostrar(g.holerite);
      },
      error: (err) => { this.ocupado.set(false); this.avisos.erro(mensagemDeErro(err)); }
    });
  }

  private desfazerPdf(empregoId: number, chave: string, nomeMes: string): void {
    this.api.desfazerHoleritePdf(empregoId, chave).subscribe({
      next: () => {
        this.avisos.sucesso(`Desfeito: o holerite de ${nomeMes} voltou ao que era antes do PDF.`);
        this.recarregarLista();
        const h = this.aberto();
        if (h) {
          this.api.holerite(h.id).subscribe({
            next: (novo) => this.mostrar(novo),
            error: () => {
              this.aberto.set(null);
              this.router.navigate([], { relativeTo: this.rota, queryParams: { holerite: null }, replaceUrl: true });
            }
          });
        }
      },
      error: (err) => this.avisos.erro(mensagemDeErro(err))
    });
  }
}
