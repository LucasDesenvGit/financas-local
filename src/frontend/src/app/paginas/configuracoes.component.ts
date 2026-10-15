import { DatePipe } from '@angular/common';
import { Component, OnInit, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ApiService } from '../core/api.service';
import { AuthService } from '../core/auth.service';
import { mensagemDeErro } from '../core/erro.interceptor';
import { Auditoria, Conexao, SincronizacaoLog } from '../core/modelos';
import { AvisosService } from '../compartilhado/avisos.component';

@Component({
  selector: 'app-configuracoes',
  imports: [FormsModule, DatePipe],
  template: `
    <header class="cabecalho-pagina"><h1>Configurações</h1></header>

    <section class="painel-bloco">
      <h2>Open Finance (via Meu Pluggy)</h2>
      <details class="guia">
        <summary>Como conectar seus bancos</summary>
        <ol>
          <li>Em <strong>meu.pluggy.ai</strong>, crie sua conta e conecte seus bancos pelo fluxo oficial do Open Finance
            (o consentimento é dado no app do seu banco).</li>
          <li>Em <strong>dashboard.pluggy.ai</strong>, crie uma conta de desenvolvedor, adicione o conector <em>MeuPluggy</em>
            e crie uma aplicação de desenvolvimento. Copie o <strong>Client ID</strong> e o <strong>Client Secret</strong>.</li>
          <li>Na aplicação de demonstração do dashboard, conecte o <em>MeuPluggy</em> e autorize — uma vez para cada banco.
            Cada autorização gera um <strong>Item ID</strong> (exibido no dashboard).</li>
          <li>Preencha as credenciais abaixo, clique em <em>Testar</em> e adicione cada Item ID em <em>Conexões</em>.</li>
        </ol>
        <p class="texto-suave">O Meu Pluggy atualiza os dados diariamente. O consentimento do Open Finance vale até 12 meses;
          o painel avisa quando estiver perto de expirar.</p>
      </details>

      <form (ngSubmit)="salvarCredenciais()" class="formulario">
        <div class="grade-campos">
          <label>Client ID <input name="clientId" [(ngModel)]="clientId" required maxlength="100" autocomplete="off"></label>
          <label>Client Secret
            <input name="clientSecret" type="password" [(ngModel)]="clientSecret" maxlength="200" autocomplete="new-password"
              [placeholder]="secretConfigurado ? '•••••••• (guardado criptografado — deixe vazio para manter)' : ''">
          </label>
          <label>Sincronizar a cada (horas) <input type="number" name="intervalo" [(ngModel)]="intervaloHoras" min="1" max="48"></label>
          <label>Reler últimos (dias) <input type="number" name="dias" [(ngModel)]="diasRetroativos" min="1" max="365"></label>
        </div>
        <label class="checkbox"><input type="checkbox" name="habilitada" [(ngModel)]="habilitada"> Sincronização automática habilitada</label>
        <div class="rodape-form">
          @if (secretConfigurado) { <button type="button" class="botao botao-perigo" (click)="removerSecret()">Apagar secret</button> }
          <span class="cresce"></span>
          <button type="button" class="botao" (click)="testar()">Testar conexão</button>
          <button class="botao botao-primario">Salvar</button>
        </div>
      </form>
    </section>

    <section class="painel-bloco">
      <div class="bloco-titulo">
        <h2>Conexões</h2>
        <button type="button" class="botao" (click)="sincronizarTudo()" [disabled]="ocupado()">⟳ Sincronizar agora</button>
      </div>
      <form (ngSubmit)="adicionar()" class="linha-flex">
        <input name="itemId" [(ngModel)]="novoItemId" placeholder="Item ID (ex.: 3f6c1a2b-…)" class="cresce" maxlength="64" required>
        <button class="botao botao-primario" [disabled]="ocupado() || !novoItemId.trim()">Adicionar conexão</button>
      </form>

      <div class="tabela-rolagem">
        <table class="tabela">
          <thead><tr><th>Instituição</th><th>Status</th><th>Atualizado no banco</th><th>Última sincronização</th><th>Consentimento até</th><th></th></tr></thead>
          <tbody>
            @for (c of conexoes(); track c.id) {
              <tr [class.ignorada]="!c.ativa">
                <td>
                  <span class="ponto" [style.background]="c.corPrimaria ?? '#64748b'"></span>
                  {{ c.nomeInstituicao ?? 'Instituição' }} <small class="texto-suave">{{ c.quantidadeContas }} conta(s)</small>
                  @if (c.ultimoErro) { <div class="erro pequeno">{{ c.ultimoErro }}</div> }
                </td>
                <td><span class="etiqueta" [class.etiqueta-sucesso]="c.status === 'UPDATED'"
                  [class.etiqueta-alerta]="c.status !== 'UPDATED'">{{ c.status ?? '—' }}</span></td>
                <td>{{ c.ultimaAtualizacaoInstituicao ? (c.ultimaAtualizacaoInstituicao + 'Z' | date: 'dd/MM/yy HH:mm') : '—' }}</td>
                <td>{{ c.ultimaSincronizacaoEm ? (c.ultimaSincronizacaoEm + 'Z' | date: 'dd/MM/yy HH:mm') : '—' }}</td>
                <td>{{ c.consentimentoExpiraEm ? (c.consentimentoExpiraEm + 'Z' | date: 'dd/MM/yyyy') : '—' }}</td>
                <td class="direita nowrap">
                  <button type="button" class="link" (click)="sincronizar(c)" [disabled]="ocupado()">sincronizar</button>
                  <button type="button" class="link" (click)="alternar(c)">{{ c.ativa ? 'pausar' : 'reativar' }}</button>
                  <button type="button" class="link perigo" (click)="remover(c)">remover</button>
                </td>
              </tr>
            } @empty { <tr><td colspan="6" class="vazio">Nenhuma conexão. Adicione o Item ID gerado no dashboard da Pluggy.</td></tr> }
          </tbody>
        </table>
      </div>

      <details>
        <summary>Histórico de sincronizações</summary>
        <div class="tabela-rolagem">
          <table class="tabela">
            <thead><tr><th>Início</th><th>Instituição</th><th>Origem</th><th>Resultado</th><th class="direita">Novas</th><th class="direita">Atualizadas</th><th>Mensagem</th></tr></thead>
            <tbody>
              @for (l of logs(); track l.id) {
                <tr>
                  <td class="nowrap">{{ l.iniciadoEm + 'Z' | date: 'dd/MM/yy HH:mm' }}</td>
                  <td>{{ l.nomeInstituicao ?? '—' }}</td>
                  <td>{{ l.origem }}</td>
                  <td [class.positivo]="l.sucesso" [class.negativo]="!l.sucesso">{{ l.sucesso ? 'ok' : 'falha' }}</td>
                  <td class="direita">{{ l.transacoesNovas }}</td>
                  <td class="direita">{{ l.transacoesAtualizadas }}</td>
                  <td class="texto-suave">{{ l.mensagem }}</td>
                </tr>
              } @empty { <tr><td colspan="7" class="vazio">Sem histórico.</td></tr> }
            </tbody>
          </table>
        </div>
      </details>
    </section>

    <div class="grade-2">
      <section class="painel-bloco">
        <h2>Alterar senha</h2>
        <form (ngSubmit)="alterarSenha()" #fs="ngForm" class="formulario">
          <label>Senha atual <input type="password" name="atual" [(ngModel)]="senhaAtual" required autocomplete="current-password"></label>
          <label>Nova senha (mín. 8 caracteres) <input type="password" name="nova" [(ngModel)]="novaSenha" required minlength="8" autocomplete="new-password"></label>
          <label>Confirmar nova senha <input type="password" name="conf" [(ngModel)]="confirmacao" required autocomplete="new-password"></label>
          <p class="texto-suave">Ao trocar a senha, todas as outras sessões abertas são encerradas.</p>
          <div class="rodape-form"><span class="cresce"></span>
            <button class="botao botao-primario" [disabled]="fs.invalid">Alterar senha</button></div>
        </form>
      </section>

      <section class="painel-bloco">
        <h2>Acessos recentes</h2>
        <ul class="lista-simples">
          @for (a of auditoria(); track a.id) {
            <li>
              <span class="data-curta nowrap">{{ a.dataHora + 'Z' | date: 'dd/MM HH:mm' }}</span>
              <span class="cresce" [class.negativo]="a.evento.includes('Falha') || a.evento.includes('Bloqueado')">{{ a.evento }}</span>
              <span class="texto-suave">{{ a.ip }}</span>
            </li>
          } @empty { <li class="vazio">Sem registros.</li> }
        </ul>
      </section>
    </div>
  `
})
export class ConfiguracoesComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly auth = inject(AuthService);
  private readonly avisos = inject(AvisosService);

  readonly conexoes = signal<Conexao[]>([]);
  readonly logs = signal<SincronizacaoLog[]>([]);
  readonly auditoria = signal<Auditoria[]>([]);
  readonly ocupado = signal(false);

  clientId = '';
  clientSecret = '';
  secretConfigurado = false;
  intervaloHoras = 6;
  diasRetroativos = 30;
  habilitada = true;
  novoItemId = '';
  senhaAtual = '';
  novaSenha = '';
  confirmacao = '';

  ngOnInit(): void {
    this.api.configuracaoPluggy().subscribe({
      next: (c) => {
        this.clientId = c.clientId ?? '';
        this.secretConfigurado = c.secretConfigurado;
        this.intervaloHoras = c.intervaloHoras;
        this.diasRetroativos = c.diasRetroativos;
        this.habilitada = c.habilitada;
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
    this.carregarConexoes();
    this.api.auditoria().subscribe((a) => this.auditoria.set(a.slice(0, 15)));
  }

  carregarConexoes(): void {
    this.api.conexoes().subscribe((c) => this.conexoes.set(c));
    this.api.logsSincronizacao().subscribe((l) => this.logs.set(l));
  }

  salvarCredenciais(): void {
    this.api.salvarConfiguracaoPluggy({
      clientId: this.clientId.trim(),
      clientSecret: this.clientSecret.trim() || null,
      intervaloHoras: Number(this.intervaloHoras) || 6,
      diasRetroativos: Number(this.diasRetroativos) || 30,
      habilitada: this.habilitada
    }).subscribe({
      next: () => {
        if (this.clientSecret.trim()) this.secretConfigurado = true;
        this.clientSecret = '';
        this.avisos.sucesso('Configuração salva.');
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  removerSecret(): void {
    if (!confirm('Apagar o Client Secret? A sincronização vai parar até você informar um novo.')) return;
    this.api.removerSecretPluggy().subscribe({
      next: () => {
        this.secretConfigurado = false;
        this.avisos.sucesso('Client Secret apagado.');
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  testar(): void {
    this.api.testarPluggy().subscribe({
      next: (r) => this.avisos.sucesso(r.mensagem),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  adicionar(): void {
    const id = this.novoItemId.trim();
    if (!id) return;
    this.ocupado.set(true);
    this.avisos.info('Validando e fazendo a primeira carga (pode levar alguns minutos)…');
    this.api.adicionarConexao(id).subscribe({
      next: (r) => {
        this.ocupado.set(false);
        this.novoItemId = '';
        if (r.sucesso) this.avisos.sucesso(`Conexão adicionada: ${r.transacoesNovas} transação(ões) importada(s).`);
        else this.avisos.erro(r.mensagem ?? 'Conexão adicionada, mas a sincronização falhou.');
        this.carregarConexoes();
      },
      error: (e) => {
        this.ocupado.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }

  sincronizar(c: Conexao): void {
    this.executarSincronizacao(c.id);
  }

  sincronizarTudo(): void {
    this.executarSincronizacao(undefined);
  }

  alternar(c: Conexao): void {
    this.api.alterarConexaoAtiva(c.id, !c.ativa).subscribe({
      next: () => this.carregarConexoes(),
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  remover(c: Conexao): void {
    const nome = c.nomeInstituicao ?? 'esta conexão';
    if (!confirm(`Remover ${nome} e APAGAR todas as contas, transações e investimentos importados dela?`)) return;
    this.api.removerConexao(c.id).subscribe({
      next: () => {
        this.avisos.sucesso('Conexão removida.');
        this.carregarConexoes();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  alterarSenha(): void {
    if (this.novaSenha !== this.confirmacao) {
      this.avisos.erro('As senhas não conferem.');
      return;
    }
    this.api.alterarSenha(this.senhaAtual, this.novaSenha).subscribe({
      next: () => {
        this.senhaAtual = this.novaSenha = this.confirmacao = '';
        this.avisos.sucesso('Senha alterada.');
        void this.auth.carregarEstado();
      },
      error: (e) => this.avisos.erro(mensagemDeErro(e))
    });
  }

  private executarSincronizacao(conexaoId: number | undefined): void {
    this.ocupado.set(true);
    this.api.sincronizar(conexaoId).subscribe({
      next: (r) => {
        this.ocupado.set(false);
        if (r.sucesso) this.avisos.sucesso(`Sincronizado: ${r.transacoesNovas} nova(s), ${r.transacoesAtualizadas} atualizada(s).`);
        else this.avisos.erro(r.mensagem ?? 'Falha na sincronização.');
        this.carregarConexoes();
      },
      error: (e) => {
        this.ocupado.set(false);
        this.avisos.erro(mensagemDeErro(e));
      }
    });
  }
}
