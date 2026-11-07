#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - 3) Compilacao e publicacao (Kestrel, sem IIS)
#
#  - Compila o front-end Angular e o back-end em Go (o site, sem o .NET
#    desde a #86)
#  - Publica em publicado/api: o executavel financas-go, o appsettings.json
#    e o front-end no wwwroot
#  - Gera/reaproveita um certificado HTTPS local (PFX) e escreve o
#    appsettings.Production.json (com caminhos absolutos, ja que o processo
#    pode rodar como servico a partir de outro diretorio de trabalho)
#  - Registra e inicia um servico nativo do sistema operacional
#    (systemd --user no Linux, sc.exe no Windows)
#
# Rode de novo sempre que atualizar o codigo: a publicacao nao apaga dados.
#
# Uso:
#   scripts/geral/3-publicar-atualizar.sh
#   scripts/geral/3-publicar-atualizar.sh --pular-build
# =====================================================================
source "$(dirname "${BASH_SOURCE[0]}")/comum.sh"

pular_build=0
while [[ $# -gt 0 ]]; do
    case "$1" in
        --pular-build) pular_build=1 ;;
        *) erro_sai "Argumento desconhecido: $1 (use --pular-build)" ;;
    esac
    shift
done

# #86: o .NET saiu; o servico e sempre o Go. A escolha da #85 (dados/backend) nao vale mais.
rm -f "$Dados/backend"

pasta_front="$Raiz/src/frontend"
preparo="$PastaPublicado/preparo"
destino="$PastaPublicado/api"
nome_servico='FinancasLocal'

# node e go (portateis ou do PATH) valem tambem com --pular-build: o appsettings.Production.json e gerado pelo
# node, e o certificado, pelo executavel do Go.
configurar_ambiente_build

# #614: sem administrador, o servico do Windows nao para, nao troca e nao sobe, e o site fica fora do ar com a
# publicacao nova pela metade. Para antes de mexer em qualquer coisa.
if [[ "$SO" == windows ]] && ! net session >/dev/null 2>&1; then
    erro_sai 'Este Git Bash nao esta como Administrador, e o servico do Windows precisa disso. Nada foi mudado.
       Feche e abra o Git Bash com o botao direito > "Executar como administrador" (ou use o instalar-windows.cmd).'
fi

# ------------------------------------------------------------------ Build
nome_go=financas-go; [[ "$SO" == windows ]] && nome_go=financas-go.exe
if [[ "$pular_build" -eq 0 ]]; then
    etapa 'Compilando o front-end (Angular)'
    # O Windows nao apaga um .exe em execucao: com um "npm start" aberto (ng serve + esbuild.exe de
    # src/frontend/node_modules), o "npm ci" falha com EPERM. Encerra so o que roda de dentro dessa pasta
    # (mesma regra do Encerrar-UsoDoFrontEnd do instalar-windows.ps1). No Linux, apagar arquivo em uso funciona.
    if [[ "$SO" == windows ]]; then
        MODULOS_FRONT="$(cygpath -w "$pasta_front/node_modules")" powershell.exe -NoProfile -NonInteractive -Command '
            $modulos = $env:MODULOS_FRONT.ToLowerInvariant()
            $emUso = @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
                ($_.ExecutablePath -and $_.ExecutablePath.ToLowerInvariant().StartsWith($modulos)) -or
                ($_.CommandLine -and $_.CommandLine.ToLowerInvariant().Contains($modulos)) })
            if ($emUso.Count -eq 0) { exit 0 }
            Write-Host "    !!  Encerrando $($emUso.Count) processo(s) que usam o front-end (ex.: um npm start aberto); sem isso o npm ci falha:" -ForegroundColor Yellow
            foreach ($p in $emUso) {
                Write-Host "    !!    PID $($p.ProcessId): $($p.Name)" -ForegroundColor Yellow
                Stop-Process -Id $p.ProcessId -Force -ErrorAction SilentlyContinue
            }
            Start-Sleep -Seconds 1' || aviso 'Nao deu para conferir processos usando o front-end; se o npm ci falhar com EPERM, feche o "npm start".'
    fi
    (cd "$pasta_front" && npm ci --no-audit --no-fund && npm run build)
    ok 'front-end compilado'

    # #86: sem a API em .NET. A publicacao e o executavel do Go, o appsettings.json dele e o front-end.
    rm -rf "$preparo"
    garantir_pasta "$preparo"
    cp "$Raiz/src/backend/appsettings.json" "$preparo/appsettings.json"

    etapa 'Compilando o back-end em Go'
    command -v go >/dev/null 2>&1 || erro_sai 'Go nao encontrado. Rode antes o 1-preparar-ambiente.sh.'
    versao_sistema="$(sed -n 's:.*<Version>\(.*\)</Version>.*:\1:p' "$Raiz/Directory.Build.props" | head -n1)"
    (cd "$Raiz/src/backend" && go build -trimpath \
        -ldflags "-s -w -X github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/versao.Atual=$versao_sistema" \
        -o "$(caminho_nativo "$preparo/$nome_go")" ./cmd/financas)
    ok "back-end em Go compilado ($(go env GOVERSION))"

    dist_front="$pasta_front/dist/financas-web/browser"
    [[ -f "$dist_front/index.html" ]] || erro_sai "Saida do Angular nao encontrada em $dist_front"
    wwwroot="$preparo/wwwroot"
    garantir_pasta "$wwwroot"
    cp -a "$dist_front/." "$wwwroot/"
    ok 'front-end incorporado ao wwwroot'
elif [[ ! -d "$preparo" ]]; then
    erro_sai 'Nao ha build anterior em publicado/preparo. Rode sem --pular-build.'
fi

# ------------------------------------------------------------------ Certificado HTTPS
etapa 'Certificado HTTPS local'
garantir_pasta "$PastaChaves"
pfx="$PastaChaves/financaslocal.pfx"
config_atual="$destino/appsettings.Production.json"

# A senha do PFX so e guardada dentro do appsettings.Production.json gerado
# (nunca impressa na tela). Para reaproveitar um PFX ja existente sem gerar
# um novo a cada publicacao, lemos a senha do appsettings.Production.json ja
# publicado anteriormente (se existir).
obter_senha_atual() {
    [[ -f "$config_atual" ]] || return 1
    node -e '
try {
    const dados = JSON.parse(require("fs").readFileSync(process.argv[1], "utf8").replace(/^\uFEFF/, ""));
    const senha = dados.Kestrel.Endpoints.Https.Certificate.Password;
    if (!senha) process.exit(1);
    process.stdout.write(senha);
} catch { process.exit(1); }
' "$(caminho_nativo "$config_atual")" 2>/dev/null
}

# passo_a_passo_certificado: mostra os comandos para confiar na parte publica do certificado (sem a
# chave, no .pem) na distribuicao detectada e no Chrome/Chromium e no Firefox (#300).
# So mostra: mexer no repositorio de certificados do sistema pede sudo e e decisao de quem instala.
passo_a_passo_certificado() {
    local id=""
    ok "parte publica do certificado (sem a chave) em $pem"
    [[ "$SO" == linux && -r /etc/os-release ]] && id="$(. /etc/os-release; echo "${ID:-} ${ID_LIKE:-}")"
    echo '    Para o navegador confiar no site (uma vez):'
    case "$id" in
        *debian*|*ubuntu*)
            echo "      sudo cp \"$pem\" /usr/local/share/ca-certificates/financaslocal.crt && sudo update-ca-certificates" ;;
        *fedora*|*rhel*|*centos*)
            echo "      sudo cp \"$pem\" /etc/pki/ca-trust/source/anchors/financaslocal.pem && sudo update-ca-trust" ;;
        *arch*)
            echo "      sudo trust anchor --store \"$pem\"" ;;
        *)
            echo "      Sistema: adicione $pem aos certificados confiaveis da sua distribuicao." ;;
    esac
    echo '      Chrome/Chromium (pacote libnss3-tools ou nss-tools):'
    echo "      certutil -d sql:\$HOME/.pki/nssdb -A -t \"P,,\" -n \"Financas Local\" -i \"$pem\""
    echo "      Firefox: Configuracoes > Privacidade e seguranca > Certificados > Ver certificados > Servidores > Importar: $pem"
    echo '    Detalhes: docs/INSTALACAO-PELO-CODIGO.md, secao "O navegador alerta sobre o certificado".'
}

senha_pfx=""
if [[ -f "$pfx" ]]; then
    senha_pfx="$(obter_senha_atual || true)"
fi
if [[ -z "$senha_pfx" ]]; then
    if command -v openssl >/dev/null 2>&1; then
        senha_pfx="$(openssl rand -base64 24)"
    else
        senha_pfx="$(head -c32 /dev/urandom | base64)"
    fi
fi

# #84: quem cuida do certificado e o executavel do Go, sem o .NET: reaproveita o PFX que abre com a senha e
# vale por mais de 30 dias, e gera outro no caso contrario. A senha vai pela entrada padrao, nao na linha de comando.
pem="${pfx%.pfx}.pem"
resultado_certificado="$(printf '%s\n' "$senha_pfx" | "$preparo/$nome_go" gerar-certificado -pfx "$(caminho_nativo "$pfx")")" \
    || erro_sai 'Nao foi possivel gerar o certificado HTTPS.'
if [[ "$resultado_certificado" == reaproveitado ]]; then
    ok 'certificado existente reaproveitado'
else
    ok "certificado gerado em $pfx"
fi

if [[ "$SO" == windows ]] && certutil.exe -addstore -f Root "$(caminho_nativo "$pem")" >/dev/null 2>&1; then
    ok 'certificado marcado como confiavel neste computador'
else
    aviso 'A instalacao nao mexe nos certificados confiaveis do sistema: isso pede sudo e e decisao de quem instala.'
    aviso 'Sem isso, o navegador mostra um aviso de seguranca ao abrir o site.'
    passo_a_passo_certificado
fi

# ------------------------------------------------------------------ Configuracao de producao
etapa 'Gerando appsettings.Production.json'
# Caminhos gravados no json no formato nativo (caminho_nativo, em comum.sh); o
# JSON.stringify do Node escapa corretamente qualquer caractere especial.
PASTA_SQL="$(caminho_nativo "$PastaSql")" \
PASTA_CHAVES="$(caminho_nativo "$PastaChaves")" \
PFX_CAMINHO="$(caminho_nativo "$pfx")" \
PFX_SENHA="$senha_pfx" \
DESTINO_CONFIG="$(caminho_nativo "$preparo/appsettings.Production.json")" \
node -e '
const e = process.env;
const config = {
    Banco: { Caminho: e.PASTA_SQL + "/FinancasLocal.db" },
    Seguranca: { PastaChaves: e.PASTA_CHAVES },
    Kestrel: {
        Endpoints: {
            Https: {
                Certificate: { Path: e.PFX_CAMINHO, Password: e.PFX_SENHA },
            },
        },
    },
};
require("fs").writeFileSync(e.DESTINO_CONFIG, JSON.stringify(config, null, 2) + "\n", "utf8");
'
ok 'configuracao gerada'

# ------------------------------------------------------------------ Copia dos arquivos
# O Windows trava o executavel em uso: o servico tem de parar antes da copia.
if [[ "$SO" == windows ]] && sc.exe query "$nome_servico" >/dev/null 2>&1; then
    sc.exe stop "$nome_servico" >/dev/null 2>&1 || true
    for _ in $(seq 1 15); do
        sc.exe query "$nome_servico" 2>/dev/null | grep -q STOPPED && break
        sleep 2
    done
fi

etapa "Publicando em $destino"
garantir_pasta "$destino"
# Ha uma breve janela de indisponibilidade durante a copia (poucos segundos).
# Sem IIS/nginx na frente para enfileirar requisicoes, isso e aceitavel aqui.
if command -v rsync >/dev/null 2>&1; then
    rsync -a --delete "$preparo/" "$destino/"
else
    cp -a "$preparo/." "$destino/"
    while IFS= read -r -d '' arquivo; do
        relativo="${arquivo#"$destino"/}"
        [[ -e "$preparo/$relativo" ]] || rm -rf "$arquivo"
    done < <(find "$destino" -mindepth 1 -print0)
fi
ok 'arquivos publicados'

# ------------------------------------------------------------------ Porta ocupada (#300)
# Outro programa na porta impede o site de subir. No Windows o servico ja parou para a copia; no Linux
# o proprio servico pode estar no ar (vai ser reiniciado), e esse nao conta.
porta_site=5443
ocupante="$(quem_usa_a_porta "$porta_site")"
if [[ -n "$ocupante" && "$SO" == linux ]] && command -v systemctl >/dev/null 2>&1; then
    pid_servico="$(systemctl --user show -p MainPID --value financaslocal.service 2>/dev/null || true)"
    [[ -n "$pid_servico" && "$pid_servico" != 0 && "$ocupante" == *"(PID $pid_servico)"* ]] && ocupante=""
fi
if [[ -n "$ocupante" ]]; then
    aviso "A porta $porta_site esta em uso por $ocupante: o site nao vai subir enquanto ela estiver ocupada."
    aviso 'Feche esse programa (ou pare o servico dele) e rode esta etapa de novo.'
fi

# ------------------------------------------------------------------ Servico nativo do SO
etapa "Registrando o servico $nome_servico"
# #84: o ponto unico que diz o que o servico executa: o Go sozinho, que aplica o esquema e roda as rotinas de
# partida (desde a #86, sem o .NET).
exe_servico="$destino/financas-go"
[[ "$SO" == windows ]] && exe_servico="$destino/financas-go.exe"
args_servico="-aplicar-esquema -rotinas-de-partida"

servico_registrado=0   # 1 quando o servico foi registrado e iniciado; sem isso (ex.: conteiner sem systemd), nada a testar
registrar_servico() {
case "$SO" in
    linux)
        unidade_dir="$HOME/.config/systemd/user"
        garantir_pasta "$unidade_dir"
        unidade="$unidade_dir/financaslocal.service"
        cat > "$unidade" <<EOF
[Unit]
Description=FinancasLocal
After=network.target

[Service]
ExecStart=$exe_servico $args_servico
WorkingDirectory=$destino
Restart=on-failure
Environment=ASPNETCORE_ENVIRONMENT=Production

[Install]
WantedBy=default.target
EOF
        if command -v systemctl >/dev/null 2>&1; then
            systemctl --user daemon-reload
            # #85: a volta automatica reinicia o servico logo depois de uma tentativa; sem zerar a contagem, o systemd
            # recusa com start-limit-hit (por padrao, 5 inicios em 10 s).
            systemctl --user reset-failed financaslocal.service 2>/dev/null || true
            if systemctl --user is-active --quiet financaslocal.service; then
                systemctl --user restart financaslocal.service
                ok 'servico systemd (usuario) atualizado e reiniciado'
            else
                systemctl --user enable --now financaslocal.service
                ok 'servico systemd (usuario) registrado e iniciado'
            fi
            servico_registrado=1
            # #277: o systemd do usuario so sobe o site com a sessao aberta; sem login (o computador ligado sem
            # ninguem entrar), so com o linger. O gerenciador tambem avisa.
            if command -v loginctl >/dev/null 2>&1 && \
               [[ "$(loginctl show-user "$(id -un)" -p Linger --value 2>/dev/null || true)" == no ]]; then
                aviso 'O site sobe quando voce entra na sessao. Para subir com o computador, mesmo sem login:'
                aviso "  sudo loginctl enable-linger $(id -un)"
            fi
        else
            aviso "systemctl nao encontrado; unidade escrita em $unidade mas nao habilitada automaticamente."
        fi
        ;;
    osx)
        aviso 'Registro automatico de servico (launchd) nao implementado para macOS.'
        aviso "Para rodar manualmente: $exe_servico $args_servico"
        ;;
    windows)
        # sc.exe e bem especifico sobre espacos: "chave= valor", com um unico
        # espaco apos o "=" e nenhum antes. Por isso binPath= e o valor vao
        # como argumentos separados do bash.
        exe_servico_win="$(cygpath -w "$exe_servico" 2>/dev/null || echo "$exe_servico")"
        bin_path="\"$exe_servico_win\" $args_servico"   # as opcoes do Go, sem aspas
        comando_create="sc.exe create $nome_servico binPath= \"$bin_path\" start= auto"
        comando_start="sc.exe start $nome_servico"
        # #614: falhar aqui e erro, e nao aviso: o site estaria fora do ar. A saida do
        # sc.exe vai junto, para dizer o motivo.
        if sc.exe query "$nome_servico" >/dev/null 2>&1; then
            sc.exe stop "$nome_servico" >/dev/null 2>&1 || true
            if ! saida_sc="$(sc.exe config "$nome_servico" binPath= "$bin_path" start= auto 2>&1)"; then
                erro_sai "Nao foi possivel reconfigurar o servico: $saida_sc
       Num prompt como Administrador: sc.exe config $nome_servico binPath= \"$bin_path\" start= auto && $comando_start"
            fi
        elif ! saida_sc="$(sc.exe create "$nome_servico" binPath= "$bin_path" start= auto 2>&1)"; then
            erro_sai "Nao foi possivel criar o servico: $saida_sc
       Num prompt como Administrador: $comando_create && $comando_start"
        fi
        # O sc.exe stop e assincrono: espera o servico parar de verdade (ate 30 s) antes de iniciar de novo.
        for _ in $(seq 1 30); do
            sc.exe query "$nome_servico" 2>/dev/null | grep -q STOPPED && break
            sleep 1
        done
        if saida_sc="$(sc.exe start "$nome_servico" 2>&1)"; then
            ok 'servico do Windows registrado e iniciado'
            servico_registrado=1
        else
            erro_sai "Nao foi possivel iniciar o servico: $saida_sc
       Rode manualmente, como Administrador: $comando_start"
        fi
        ;;
esac
}

registrar_servico

# ------------------------------------------------------------------ Teste
etapa 'Testando'
url_teste='https://127.0.0.1:5443/api/auth/estado'
# testar_site <tentativas>: o site responde em ate 2 s x tentativas (o Go sozinho tem 30 s, como pede a #85).
testar_site() {
    local tentativas="$1"
    sucesso=0
    resposta=""
    for _ in $(seq 1 "$tentativas"); do
        if resposta="$(curl -k -fsS "$url_teste" 2>/dev/null)"; then
            sucesso=1
            return 0
        fi
        sleep 2
    done
    return 1
}
if [[ "$servico_registrado" -eq 1 ]]; then
    testar_site 15 || true
else
    testar_site 5 || true
fi
if [[ "$sucesso" -eq 1 ]]; then
    ok "API respondeu: $resposta"
else
    aviso "O site nao respondeu em $url_teste. Veja o registro do servico (Linux: journalctl --user -u financaslocal.service;"
    aviso 'Windows: Visualizador de Eventos). As copias do banco ficam em backups/. Para voltar a versao com o .NET:'
    aviso '  git checkout v-ultimo-dotnet && scripts/geral/instalar.sh'
    ocupante="$(quem_usa_a_porta 5443)"
    [[ -n "$ocupante" ]] && aviso "Quem esta na porta 5443 agora: $ocupante. Se nao for o Financas Local, feche esse programa."
fi

if [[ "$sucesso" -eq 1 ]]; then
    for velho in "$Ferramentas/dotnet" "$Ferramentas/dotnet-home" "$Ferramentas/nuget-pacotes" "$PastaPublicado/api-anterior"; do
        [[ -e "$velho" ]] || continue
        rm -rf "$velho" && ok "o que so o .NET usava saiu: $velho"
    done
fi

# #655: o backup agendado antes da #154 chama o repasse scripts/4-backup.sh. Troca pelo caminho novo, mantendo
# horario e --manter, para o repasse poder sair sem o backup parar em silencio.
backup_antigo="$Raiz/scripts/4-backup.sh"
backup_novo="$Raiz/scripts/geral/4-backup.sh"
case "$SO" in
    linux|osx)
        cron_atual="$(crontab -l 2>/dev/null || true)"
        if grep -qF "$backup_antigo " <<<"$cron_atual"; then
            if grep -qF "$backup_novo " <<<"$cron_atual"; then
                cron_novo="$(grep -vF "$backup_antigo " <<<"$cron_atual" || true)"   # ja agendado pelo novo: so tira o velho
            else
                cron_novo="${cron_atual//"$backup_antigo "/"$backup_novo "}"
            fi
            if printf '%s\n' "$cron_novo" | crontab -; then
                ok "Backup agendado passou para o caminho novo ($backup_novo)."
            else
                aviso "Nao deu para trocar o backup agendado. Rode: scripts/geral/4-backup.sh --agendar"
            fi
        fi
        ;;
    windows)
        powershell.exe -NoProfile -NonInteractive -Command '
            $t = Get-ScheduledTask -TaskName "FinancasLocal - Backup diario" -ErrorAction SilentlyContinue
            if (-not $t) { exit 0 }
            $a = $t.Actions[0]
            if ($a.Arguments -notmatch "\\scripts\\4-backup\.sh") { exit 0 }
            $a.Arguments = $a.Arguments -replace "\\scripts\\4-backup\.sh", "\scripts\geral\4-backup.sh"
            Set-ScheduledTask -TaskName $t.TaskName -TaskPath $t.TaskPath -Action $a | Out-Null
            exit 2' >/dev/null 2>&1
        case $? in
            0) ;;
            2) ok "Backup agendado passou para o caminho novo ($backup_novo)." ;;
            *) aviso "Nao deu para trocar o backup agendado. Rode: scripts/geral/4-backup.sh --agendar" ;;
        esac
        ;;
esac

if [[ "$SO" == windows ]] && reg.exe query 'HKLM\SOFTWARE\Mozilla\Mozilla Firefox' >/dev/null 2>&1; then
    aviso 'Firefox instalado: por padrao ele nao usa os certificados do Windows e mostra um aviso no site.'
    aviso 'Para aceitar: em about:config, deixe security.enterprise_roots.enabled como true (ou use o Edge/Chrome).'
fi

echo ''
echo -e "\033[32mPronto! Abra $url_teste no navegador (ajuste a porta se tiver mudado no appsettings.json).\033[0m"
