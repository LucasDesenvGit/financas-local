#!/usr/bin/env python3
"""
As regras de ouro do back-end em Go, conferidas pela CI e não só pela revisão (#457, ADR 0049 e ADR 0051). Lê os .go de
src/backend-go e falha em:
- float64 (e float32, Float64(), ParseFloat...): dinheiro, quantidade e taxa são tipos.Decimal;
- time.Now: a hora vem do relogio.Relogio (só o internal/relogio chama time.Now), e o teste usa o relogio.Fixo;
- time.Sleep em _test.go: espere por um sinal (canal, WaitGroup) ou rode dentro do synctest.Test.

Comentário e texto entre aspas não contam: só o código. Dentro de um synctest.Test, o time.Now e o time.Sleep são do
relógio de mentira e passam sozinhos. O resto só passa com uma exceção escrita, com o motivo, em
scripts/ci/regras-go-excecoes.txt, no mesmo formato do mutacao-go-equivalentes.txt:

    regra | arquivo (a partir de src/backend-go) | a linha do código, sem os espaços das pontas e sem o comentário | motivo

É pelo texto da linha, e não pelo número: mexeu na linha, a exceção deixa de valer e a regra volta a falhar para
alguém olhar de novo. Exceção que não casa com nenhuma linha também falha (a lista não acumula lixo). A exceção vale
para toda linha igual no mesmo arquivo; na dúvida, escreva a exceção para a linha mais específica.

Limites conhecidos (*julgamento*): conferência por linha, sem os tipos do Go (go/types); um apelido de import
(`import t "time"`) escapa; time.Since e time.Until, que leem o relógio por dentro, não são conferidos (fora da #457).

Uso (na CI, em src/backend-go):
  python3 ../../scripts/ci/verificar-regras-go.py
"""
import argparse
import os
import re
import sys

PASTA_DO_SCRIPT = os.path.dirname(os.path.abspath(__file__))
EXCECOES_PADRAO = os.path.join(PASTA_DO_SCRIPT, 'regras-go-excecoes.txt')

FLOAT = 'float64'
AGORA = 'time.Now'
DORMIR = 'time.Sleep'
REGRAS = (FLOAT, AGORA, DORMIR)

# A função que pega o passado para a bolha do synctest: o Test (Go 1.25+) e o Run (o antigo, do GOEXPERIMENT).
BOLHA = re.compile(r'\bsynctest\s*\.\s*(?:Test|Run)\s*\(')
PADROES = {
    # Sem fronteira de palavra de propósito: pega também ParaFloat64 e DecimalDeFloat64 (decimal → float64 é o caminho
    # por onde o dinheiro escorre sem escrever "float64"), e o ParseFloat e o FormatFloat do strconv.
    FLOAT: re.compile(r'[Ff]loat(?:32|64)|ParseFloat|FormatFloat|AppendFloat'),
    AGORA: re.compile(r'\btime\s*\.\s*Now\b'),
    DORMIR: re.compile(r'\btime\s*\.\s*Sleep\b'),
}

MENSAGENS = {
    FLOAT: ('float64 não é para dinheiro, quantidade nem taxa: use tipos.Decimal. Se é um fator ou uma pontuação que o '
            'C# também calcula em double, escreva a exceção, com o motivo, em scripts/ci/regras-go-excecoes.txt'),
    AGORA: ('time.Now() fora do internal/relogio: a hora vem de um relogio.Relogio recebido pelo construtor '
            '(relogio.Fixo no teste, relogio.Sistema em produção). Se é hora de sistema e não de negócio (medir '
            'duração, validade de certificado), escreva a exceção, com o motivo, em scripts/ci/regras-go-excecoes.txt'),
    DORMIR: ('time.Sleep em teste: não espere por tempo. Espere um sinal (canal, sync.WaitGroup, o AoEscutar do '
             'servidor...), use o testing/synctest, ou os.Chtimes para a data de um arquivo. Se o tempo é o que se '
             'testa, escreva a exceção, com o motivo, em scripts/ci/regras-go-excecoes.txt'),
}


def limpar(fonte):
    """Devolve (sem_comentarios, codigo): o texto com os comentários trocados por espaços e, no segundo, também o
    conteúdo dos textos entre aspas e dos caracteres. Os dois têm o mesmo tamanho do original e as mesmas quebras de
    linha, então a posição e a linha de um achado valem para o arquivo."""
    n = len(fonte)
    sem_comentarios, codigo = list(fonte), list(fonte)

    def apagar(lista, inicio, fim):
        for k in range(inicio, fim):
            if lista[k] != '\n':
                lista[k] = ' '

    i = 0
    while i < n:
        c = fonte[i]
        if fonte.startswith('//', i):
            fim = fonte.find('\n', i)
            fim = n if fim < 0 else fim
            apagar(sem_comentarios, i, fim)
            apagar(codigo, i, fim)
            i = fim
        elif fonte.startswith('/*', i):
            fim = fonte.find('*/', i + 2)
            fim = n if fim < 0 else fim + 2
            apagar(sem_comentarios, i, fim)
            apagar(codigo, i, fim)
            i = fim
        elif c == '`':  # texto cru: vai até a próxima crase, sem escapes, e pode ter várias linhas
            fim = fonte.find('`', i + 1)
            fim = n if fim < 0 else fim + 1
            apagar(codigo, i + 1, fim - 1)
            i = fim
        elif c in '"\'':  # texto ou caractere, com barra invertida; não passa da linha
            j = i + 1
            while j < n and fonte[j] not in (c, '\n'):
                j += 2 if fonte[j] == '\\' else 1
            apagar(codigo, i + 1, min(j, n))
            i = j + 1
        else:
            i += 1
    return ''.join(sem_comentarios), ''.join(codigo)


def dentro_de_bolha(codigo):
    """Os trechos (início, fim) dos argumentos de cada synctest.Test( ... ), achando o ) que fecha."""
    trechos = []
    for achado in BOLHA.finditer(codigo):
        profundidade, i = 1, achado.end()
        while i < len(codigo) and profundidade:
            profundidade += {'(': 1, ')': -1}.get(codigo[i], 0)
            i += 1
        trechos.append((achado.end(), i))
    return trechos


def achados_do_arquivo(caminho, fonte):
    """Lista de (regra, número da linha, texto da linha sem comentário) do que o arquivo faz de errado, sem contar
    as exceções escritas."""
    sem_comentarios, codigo = limpar(fonte)
    bolhas = dentro_de_bolha(codigo)
    linhas = sem_comentarios.split('\n')
    achados = set()
    for regra, padrao in PADROES.items():
        if regra == DORMIR and not caminho.endswith('_test.go'):
            continue
        if regra == AGORA and caminho.startswith('internal/relogio/'):
            continue
        for achado in padrao.finditer(codigo):
            if regra != FLOAT and any(inicio <= achado.start() < fim for inicio, fim in bolhas):
                continue
            numero = codigo.count('\n', 0, achado.start()) + 1
            achados.add((regra, numero, linhas[numero - 1].strip()))  # uma vez por regra e linha
    return sorted(achados, key=lambda a: (a[1], a[0]))


def ler_excecoes(caminho):
    """(regra, arquivo, linha) -> motivo."""
    excecoes, erros = {}, []
    with open(caminho, encoding='utf-8') as f:
        for n, linha in enumerate(f, 1):
            texto = linha.rstrip('\n')
            if not texto.strip() or texto.lstrip().startswith('#'):
                continue
            partes = [p.strip() for p in texto.split(' | ')]
            if len(partes) != 4 or not all(partes):
                erros.append(f'{caminho}:{n}: esperado "regra | arquivo | linha do código | motivo" (os quatro, sem '
                             f'campo vazio), veio {texto!r}')
            elif partes[0] not in REGRAS:
                erros.append(f'{caminho}:{n}: regra {partes[0]!r} desconhecida; use {", ".join(REGRAS)}')
            else:
                excecoes[(partes[0], partes[1], partes[2])] = partes[3]
    return excecoes, erros


def arquivos_go(raiz):
    for pasta, subpastas, nomes in os.walk(raiz):
        # testdata e as pastas de ferramenta não são código do back-end (o go build também as ignora)
        subpastas[:] = sorted(d for d in subpastas if d not in ('testdata', 'vendor', '.git', 'node_modules'))
        for nome in sorted(nomes):
            if nome.endswith('.go'):
                yield os.path.relpath(os.path.join(pasta, nome), raiz).replace(os.sep, '/')


def conferir(raiz, caminho_excecoes):
    """Devolve a lista de problemas, cada um (arquivo, linha, mensagem); arquivo vazio é problema da lista."""
    excecoes, erros = ler_excecoes(caminho_excecoes)
    problemas = [('', 0, e) for e in erros]
    usadas = set()
    for arquivo in arquivos_go(raiz):
        with open(os.path.join(raiz, arquivo), encoding='utf-8') as f:
            fonte = f.read()
        for regra, numero, texto in achados_do_arquivo(arquivo, fonte):
            chave = (regra, arquivo, texto)
            if chave in excecoes:
                usadas.add(chave)
            else:
                problemas.append((arquivo, numero, f'[{regra}] {MENSAGENS[regra]}. Linha: {texto}'))
    for chave in excecoes:
        if chave not in usadas:
            regra, arquivo, texto = chave
            problemas.append((arquivo, 0, f'[{regra}] a exceção não casa com nenhuma linha (a linha mudou ou o código '
                                          f'saiu): tire ou atualize em {os.path.basename(caminho_excecoes)}. Linha da '
                                          f'exceção: {texto}'))
    return problemas, len(excecoes)


def main():
    sys.stdout.reconfigure(encoding='utf-8')  # o console do Windows não é UTF-8
    ap = argparse.ArgumentParser(description=__doc__.split('\n')[1])
    ap.add_argument('--raiz', default='.', help='a pasta do back-end em Go (padrão: a atual)')
    ap.add_argument('--excecoes', default=EXCECOES_PADRAO, help='a lista de exceções')
    args = ap.parse_args()

    problemas, total = conferir(args.raiz, args.excecoes)
    no_github = os.environ.get('GITHUB_ACTIONS') == 'true'
    for arquivo, numero, mensagem in problemas:
        local = f'{arquivo}:{numero}' if numero else (arquivo or os.path.basename(args.excecoes))
        print(f'{local}: {mensagem}')
        if no_github and arquivo and numero:
            print(f'::error file=src/backend-go/{arquivo},line={numero}::{mensagem}')
    if problemas:
        print(f'\n{len(problemas)} problema(s) nas regras de ouro do Go.')
        return 1
    print(f'Regras de ouro do Go em dia ({total} exceções escritas).')
    return 0


if __name__ == '__main__':
    sys.exit(main())
