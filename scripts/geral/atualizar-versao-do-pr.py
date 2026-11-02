#!/usr/bin/env python3
"""
Resolve o conflito de versão e de CHANGELOG de um PR que trouxe o develop (#587, ADR 0039).

Todo PR sobe a versão no Directory.Build.props e escreve uma seção no topo do CHANGELOG.md; quando outro PR entra
antes, os dois arquivos conflitam. Este script, rodado no meio do `git merge origin/develop` parado nesses conflitos:
- parte dos dois arquivos do develop;
- sobe a versão do develop no mesmo nível que o PR tinha subido (o primeiro, o do meio ou o último número), nunca menos
  do que o tipo do título pede (feat → o do meio; o resto → o último);
- põe no topo a seção que o PR tinha acrescentado, com o número novo e a data de hoje.

Quem chama é o scripts/geral/atualizar-pr.sh. Uso direto (na pasta do repositório, com o merge parado):
    python3 scripts/geral/atualizar-versao-do-pr.py "<título do PR>"
"""
import datetime
import re
import subprocess
import sys

PROPS = 'Directory.Build.props'
CHANGELOG = 'CHANGELOG.md'
VERSAO = re.compile(r'<Version>\s*(\d+)\.(\d+)\.(\d+)\s*</Version>')
SECAO = re.compile(r'^## \[(\d+\.\d+\.\d+)\] — \d{4}-\d{2}-\d{2}')


class Problema(Exception):
    """O que o script não resolve sozinho: a mensagem diz o que fazer à mão."""


def versao(conteudo):
    achado = VERSAO.search(conteudo)
    if not achado:
        raise Problema(f'não achei <Version>X.Y.Z</Version> no {PROPS}')
    return tuple(int(n) for n in achado.groups())


def texto(v):
    return '.'.join(map(str, v))


def nivel_do_pr(base, pr):
    """Quanto o PR subiu a versão em relação à base dele: 0 o primeiro, 1 o do meio, 2 o último, None não subiu."""
    for i in range(3):
        if pr[i] != base[i]:
            return i if pr[i] > base[i] else None
    return None


def nivel_do_titulo(titulo):
    """O mínimo pelo tipo do título (Conventional Commits): feat sobe o do meio; o resto, o último."""
    return 1 if re.match(r'\s*feat\b', titulo) else 2


def nova_versao(develop, base, pr, titulo):
    """A versão do develop subida no nível do PR, e pelo menos no nível do título (o menor índice é a subida maior)."""
    nivel = nivel_do_titulo(titulo)
    do_pr = nivel_do_pr(base, pr)
    if do_pr is not None:
        nivel = min(nivel, do_pr)
    return tuple(develop[i] if i < nivel else develop[i] + 1 if i == nivel else 0 for i in range(3))


def secao_do_pr(diff):
    """A seção que o PR acrescentou ao CHANGELOG, das linhas do `git diff -U0 base PR -- CHANGELOG.md`."""
    linhas = diff.splitlines()
    acrescentadas = [l[1:] for l in linhas if l.startswith('+') and not l.startswith('+++')]
    removidas = [l[1:] for l in linhas if l.startswith('-') and not l.startswith('---')]
    if removidas:
        raise Problema(f'o PR também tirou ou mudou linhas do {CHANGELOG}: resolva à mão')
    while acrescentadas and not acrescentadas[-1].strip():
        acrescentadas.pop()
    if not acrescentadas:
        raise Problema(f'o PR não escreveu no {CHANGELOG}: acrescente a entrada e rode de novo')
    if not SECAO.match(acrescentadas[0]) or sum(1 for l in acrescentadas if l.startswith('## [')) != 1:
        raise Problema(f'o PR não acrescentou uma seção "## [X.Y.Z] — AAAA-MM-DD" inteira no {CHANGELOG}: resolva à mão')
    return acrescentadas


def montar_changelog(do_develop, secao, nova, hoje):
    """O CHANGELOG do develop com a seção do PR no topo, renumerada para a versão nova e datada de hoje."""
    secao = [SECAO.sub(f'## [{texto(nova)}] — {hoje:%Y-%m-%d}', secao[0])] + secao[1:]
    topo = do_develop.find('\n## [')
    if topo < 0:
        raise Problema(f'não achei nenhuma seção "## [" no {CHANGELOG} do develop')
    return do_develop[:topo + 1] + '\n'.join(secao) + '\n\n' + do_develop[topo + 1:]


def git(*argumentos):
    return subprocess.run(['git', *argumentos], check=True, capture_output=True, text=True, encoding='utf-8').stdout


def gravar(caminho, conteudo):
    with open(caminho, 'w', encoding='utf-8', newline='') as f:
        f.write(conteudo)


def main(titulo, hoje):
    base = git('merge-base', 'HEAD', 'MERGE_HEAD').strip()
    props_develop = git('show', f'MERGE_HEAD:{PROPS}')
    develop = versao(props_develop)
    nova = nova_versao(develop, versao(git('show', f'{base}:{PROPS}')), versao(git('show', f'HEAD:{PROPS}')), titulo)
    secao = secao_do_pr(git('diff', '-U0', base, 'HEAD', '--', CHANGELOG))
    gravar(PROPS, VERSAO.sub(f'<Version>{texto(nova)}</Version>', props_develop, count=1))
    gravar(CHANGELOG, montar_changelog(git('show', f'MERGE_HEAD:{CHANGELOG}'), secao, nova, hoje))
    print(f'Versão {texto(develop)} (develop) → {texto(nova)}; seção do PR no topo do {CHANGELOG}.')


if __name__ == '__main__':
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    try:
        main(sys.argv[1], datetime.date.today())
    except Problema as p:
        sys.exit(f'Não deu para resolver sozinho: {p}.')
