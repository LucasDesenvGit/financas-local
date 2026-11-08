"""
O que os scripts de scripts/ci/ e scripts/dev/ repetiam (#655): ler o <Version> do Directory.Build.props, a regra "feat
sobe o número do meio, o resto sobe o último" (ADR 0039) e a lista de arquivos que um PR mudou.

Os scripts têm hífen no nome e não se importam, mas este arquivo se importa: o Python põe a pasta do script no
sys.path, então cada script põe a pasta scripts/ci no dele antes do import:

    sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), '..'))   # scripts em scripts/ci/<assunto>/
    from comum import ler_versao
"""
import os
import re
import subprocess

RAIZ = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', '..'))
PROPS = 'Directory.Build.props'
VERSAO = re.compile(r'<Version>\s*(\d+)\.(\d+)\.(\d+)\s*</Version>')


def ler_versao(conteudo):
    """A versão (X, Y, Z) do <Version> do Directory.Build.props, ou None se não achar."""
    achado = VERSAO.search(conteudo)
    return tuple(int(n) for n in achado.groups()) if achado else None


def texto_da_versao(versao):
    return '.'.join(map(str, versao))


def nivel_do_titulo(titulo):
    """O número que o tipo do título (Conventional Commits) manda subir: 1 (o do meio) para feat; 2 (o último) para o
    resto. A quebra de verdade (o primeiro) fica a critério de quem faz o PR."""
    return 1 if re.match(r'\s*feat\b', titulo) else 2


def versao_minima(base, titulo):
    """A menor versão que um PR com este título pode ter, partindo da versão (X, Y, Z) da base."""
    if nivel_do_titulo(titulo) == 1:
        return (base[0], base[1] + 1, 0)
    return (base[0], base[1], base[2] + 1)


def arquivos_mudados(base, cwd=None, check=True):
    """Arquivos que o PR mudou em relação à base (o que o merge leva: `git diff --name-only <base>...HEAD`)."""
    saida = subprocess.run(['git', 'diff', '--name-only', f'{base}...HEAD'], cwd=cwd, check=check,
                           capture_output=True, text=True).stdout
    return [a for a in saida.splitlines() if a.strip()]
