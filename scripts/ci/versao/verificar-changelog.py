#!/usr/bin/env python3
"""
Confere o que vira CHANGELOG (ADR 0070, #294):

1. changelog.d/: um arquivo por PR, "changelog.d/<nome-da-branch>.md" (a "/" da branch vira "-"). Cada arquivo começa por
   um título de tipo ("### Funcionalidades", "### Correções" ou "### Publicação e documentação"), sem tipo repetido, e
   cada tipo tem pelo menos uma entrada ("- ..."). Sem "## [X.Y.Z]": a versão é calculada pelo merge (go run
   ./cmd/versao), e quem escreve a entrada não sabe qual vai ser. O LEIA-ME.md da pasta não é entrada.
2. No PR (com TITULO_PR e BASE): título feat ou fix traz um arquivo novo em changelog.d/. As outras (CI, testes,
   documentação interna, Dependabot) podem não ter; as notas as listam pelo título.

   Cada entrada tem até ~60 palavras para quem usa; o que passa disso (o sub-item "  - Técnico: ..." não conta) é só
   aviso, nunca barra (#722, "Simples e enxuto").

As notas dos Releases e a janela "O que mudou" do gerenciador saem destes arquivos: entrada fora do formato vira nota errada
para quem atualiza.

Uso na CI:   BASE=develop TITULO_PR="fix(api): ..." python3 scripts/ci/versao/verificar-changelog.py
Local:       python3 scripts/ci/versao/verificar-changelog.py            (sem BASE e TITULO_PR, só o item 1)

O CHANGELOG.md só o `cmd/versao -publicar` escreve (#721): este script não o confere.
"""
import io
import os
import re
import subprocess
import sys

RAIZ = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', '..', '..'))
PASTA = 'changelog.d'
LEIA_ME = 'LEIA-ME.md'
TIPOS = ('### Funcionalidades', '### Correções', '### Publicação e documentação')
# Os tipos de PR que mudam algo para quem usa: o arquivo é obrigatório.
TIPOS_COM_ENTRADA = ('feat', 'fix')
# Palavras que uma entrada pode ter para quem usa, sem o "Técnico" (aviso, não erro).
LIMITE_PALAVRAS = 60


def ler(caminho):
    with io.open(caminho, encoding='utf-8') as f:
        return f.read().replace('\r\n', '\n')


def entradas(raiz=RAIZ):
    """[(caminho relativo, texto)] dos arquivos de changelog.d/, e os problemas de lugar (subpasta, não é .md)."""
    pasta = os.path.join(raiz, PASTA)
    achadas, problemas = [], []
    if not os.path.isdir(pasta):
        return achadas, [f'a pasta {PASTA}/ não existe']
    for nome in sorted(os.listdir(pasta)):
        caminho = os.path.join(pasta, nome)
        if nome == LEIA_ME:
            continue
        if os.path.isdir(caminho):
            problemas.append(f'{PASTA}/{nome}/: subpasta (os arquivos ficam direto em {PASTA}/; a "/" da branch vira "-")')
        elif not nome.endswith('.md'):
            problemas.append(f'{PASTA}/{nome}: só arquivos .md (o que não é .md não entra no CHANGELOG)')
        else:
            achadas.append((f'{PASTA}/{nome}', ler(caminho)))
    return achadas, problemas


def problemas_da_entrada(caminho, texto):
    """O que está fora do formato num arquivo de changelog.d/."""
    linhas = [l.rstrip() for l in texto.split('\n')]
    if not any(linhas):
        return [f'{caminho}: vazio']
    achados = []
    primeira = next(l for l in linhas if l)
    if not primeira.startswith('### '):
        achados.append(f'{caminho}: começa por "{primeira[:40]}"; a primeira linha é o título do tipo ({", ".join(TIPOS)})')
    titulos, bullets = [], {}
    for linha in linhas:
        if linha.startswith('## '):
            achados.append(f'{caminho}: tem "{linha[:40]}"; sem o título de versão, o número é calculado no merge')
        elif linha.startswith('### '):
            if linha in titulos:
                achados.append(f'{caminho}: o tipo "{linha}" está repetido (junte as entradas num título só)')
            titulos.append(linha)
            bullets.setdefault(linha, 0)
        elif titulos and linha.startswith('- '):
            bullets[titulos[-1]] += 1
    for titulo in dict.fromkeys(titulos):
        if titulo not in TIPOS:
            achados.append(f'{caminho}: o tipo "{titulo}" não existe (use {", ".join(TIPOS)})')
        if not bullets[titulo]:
            achados.append(f'{caminho}: o tipo "{titulo}" não tem entrada ("- ...")')
    return achados


def avisos_da_entrada(caminho, texto):
    """Entradas ("- ...") com mais de LIMITE_PALAVRAS palavras, sem contar o sub-item "  - Técnico: ..." (e o que continua nele)."""
    entradas, atual, em_tecnico = [], None, False
    for linha in texto.split('\n'):
        if linha.startswith('#'):
            atual = None
        elif linha.startswith('- '):
            atual, em_tecnico = [linha], False
            entradas.append(atual)
        elif re.match(r'\s+- Técnico:', linha):
            em_tecnico = True
        elif atual is not None and not em_tecnico:
            atual.append(linha)
    avisos = []
    for linhas in entradas:
        palavras = len(' '.join([linhas[0][2:]] + linhas[1:]).split())
        if palavras > LIMITE_PALAVRAS:
            avisos.append(f'{caminho}: a entrada "{linhas[0][:50]}" tem {palavras} palavras para quem usa (o limite é ~{LIMITE_PALAVRAS}); '
                          f'encurte e ponha o detalhe numa linha "  - Técnico: ..." (modelo no {PASTA}/{LEIA_ME})')
    return avisos


def tipo_do_titulo(titulo):
    achado = re.match(r'\s*(\w+)', titulo or '')
    return achado.group(1) if achado else ''


def arquivos_novos(base, raiz=RAIZ):
    """Arquivos de changelog.d/ que o PR acrescentou em relação à base (o que o merge leva)."""
    saida = subprocess.run(['git', 'diff', '--name-only', '--diff-filter=A', f'{base}...HEAD', '--', f'{PASTA}/'], cwd=raiz,
                           check=True, capture_output=True, text=True).stdout
    return [a for a in saida.splitlines() if a.endswith('.md') and os.path.basename(a) != LEIA_ME]


def problemas_do_pr(titulo, novos):
    """Título feat ou fix sem arquivo novo em changelog.d/."""
    tipo = tipo_do_titulo(titulo)
    if tipo in TIPOS_COM_ENTRADA and not novos:
        return [f'PR "{tipo}": falta o arquivo changelog.d/<nome-da-branch>.md com a entrada (modelo no '
                f'{PASTA}/{LEIA_ME}). Só PR feat e fix precisa; CI, testes e documentação interna não.']
    return []


def main():
    erros = []
    arquivos, erros_de_lugar = entradas()
    erros += erros_de_lugar
    for caminho, conteudo in arquivos:
        erros += problemas_da_entrada(caminho, conteudo)
        for aviso in avisos_da_entrada(caminho, conteudo):
            print(f'::warning title=Entrada longa::{aviso}')
    base, titulo = os.environ.get('BASE'), os.environ.get('TITULO_PR')
    if base and titulo is not None:
        erros += problemas_do_pr(titulo, arquivos_novos(f'origin/{base}'))
    if erros:
        print('changelog.d/ fora do formato:')
        for e in erros:
            print(f'  - {e}')
        return 1
    print(f'OK: {len(arquivos)} arquivo(s) em {PASTA}/.')
    return 0


if __name__ == '__main__':
    sys.exit(main())
