#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - 3) Compilacao e publicacao (Kestrel, sem IIS)
#
#  - Compila o front-end Angular, a API .NET e o back-end em Go (que roda
#    atras do .NET, como processo filho dele: #231)
#  - Publica em publicado/api, com o front-end incorporado ao wwwroot da API
#    e o executavel financas-go ao lado da API
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
for arg in "$@"; do
    case "$arg" in
        --pular-build) pular_build=1 ;;
        *) erro_sai "Argumento desconhecido: $arg (use --pular-build)" ;;
    esac
done

pasta_front="$Raiz/src/frontend"
projeto_api="$Raiz/src/backend/Financas.Api/Financas.Api.csproj"
preparo="$PastaPublicado/preparo"
destino="$PastaPublicado/api"
nome_servico='FinancasLocal'

# dotnet e node (portateis ou do PATH) valem tambem com --pular-build: o
# certificado usa o dotnet e o appsettings.Production.json e gerado pelo node.
configurar_ambiente_build

if [[ "$SO" == windows ]] && ! net session >/dev/null 2>&1; then
    aviso 'Este Git Bash nao esta como Administrador: a publicacao segue, mas o servico do Windows'
    aviso 'provavelmente nao sera registrado/reiniciado. Para evitar, feche e abra o Git Bash como Administrador.'
fi

# ------------------------------------------------------------------ Build
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

    etapa 'Compilando a API (.NET)'
    rm -rf "$preparo"
    dotnet publish "$projeto_api" -c Release -o "$preparo" --nologo
    ok 'API compilada'

    # #231 (ADR 0049): o Go roda atras do .NET, como processo filho; o executavel fica ao lado da API, onde o
    # .NET o procura (Go:Executavel). A versao e a do Directory.Build.props, como a da API.
    etapa 'Compilando o back-end em Go'
    command -v go >/dev/null 2>&1 || erro_sai 'Go nao encontrado. Rode antes o 1-preparar-ambiente.sh.'
    versao_sistema="$(sed -n 's:.*<Version>\(.*\)</Version>.*:\1:p' "$Raiz/Directory.Build.props" | head -n1)"
    nome_go=financas-go; [[ "$SO" == windows ]] && nome_go=financas-go.exe
    (cd "$Raiz/src/backend-go" && go build -trimpath \
        -ldflags "-s -w -X github.com/LucasDesenvGit/financas-local/src/backend-go/internal/versao.Atual=$versao_sistema" \
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

# pfx_valido_por_mais_de_30_dias senha
pfx_valido_por_mais_de_30_dias() {
    [[ -f "$pfx" ]] || return 1
    command -v openssl >/dev/null 2>&1 || return 1
    local venc_str venc_epoch limite_epoch
    venc_str="$(openssl pkcs12 -in "$pfx" -passin pass:"$1" -nokeys -clcerts 2>/dev/null | openssl x509 -noout -enddate 2>/dev/null | cut -d= -f2)"
    [[ -n "$venc_str" ]] || return 1
    venc_epoch="$(date -d "$venc_str" +%s 2>/dev/null || date -j -f '%b %d %T %Y %Z' "$venc_str" +%s 2>/dev/null || echo '')"
    [[ -n "$venc_epoch" ]] || return 1
    limite_epoch=$(( $(date +%s) + 30*24*60*60 ))
    [[ "$venc_epoch" -gt "$limite_epoch" ]]
}

# passo_a_passo_certificado: salva a parte publica do certificado (sem a chave) num .pem e mostra os
# comandos para confiar nele na distribuicao detectada e no Chrome/Chromium e no Firefox (#300).
# So mostra: mexer no repositorio de certificados do sistema pede sudo e e decisao de quem instala.
passo_a_passo_certificado() {
    local pem="${pfx%.pfx}.pem" id=""
    if command -v openssl >/dev/null 2>&1 \
        && openssl pkcs12 -in "$pfx" -passin pass:"$senha_pfx" -nokeys -clcerts 2>/dev/null \
            | openssl x509 -out "$pem" 2>/dev/null; then
        ok "parte publica do certificado (sem a chave) em $pem"
    else
        aviso "Sem o openssl para separar a parte publica do certificado; instale o openssl e rode de novo."
        return 0
    fi
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

if [[ -n "$senha_pfx" ]] && pfx_valido_por_mais_de_30_dias "$senha_pfx"; then
    ok 'certificado existente reaproveitado'
else
    etapa 'Gerando certificado HTTPS de desenvolvimento'
    if command -v openssl >/dev/null 2>&1; then
        senha_pfx="$(openssl rand -base64 24)"
    else
        senha_pfx="$(head -c32 /dev/urandom | base64)"
    fi
    rm -f "$pfx"
    dotnet dev-certs https -ep "$(caminho_nativo "$pfx")" -p "$senha_pfx" >/dev/null
    ok "certificado gerado em $pfx"
fi

if dotnet dev-certs https --trust >/dev/null 2>&1; then
    ok 'certificado marcado como confiavel neste computador'
else
    aviso 'Nao foi possivel confiar automaticamente no certificado (normal em varias distribuicoes Linux).'
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
# O Windows trava as DLLs em uso: o servico tem de parar antes da copia.
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
if [[ -x "$Ferramentas/dotnet/dotnet" ]]; then
    dotnet_exec="$Ferramentas/dotnet/dotnet"
elif [[ -x "$Ferramentas/dotnet/dotnet.exe" ]]; then
    dotnet_exec="$Ferramentas/dotnet/dotnet.exe"
else
    dotnet_exec="$(command -v dotnet)"
fi
dll_destino="$destino/Financas.Api.dll"

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
ExecStart=$dotnet_exec $dll_destino
WorkingDirectory=$destino
Restart=on-failure
Environment=ASPNETCORE_ENVIRONMENT=Production

[Install]
WantedBy=default.target
EOF
        if command -v systemctl >/dev/null 2>&1; then
            systemctl --user daemon-reload
            if systemctl --user is-active --quiet financaslocal.service; then
                systemctl --user restart financaslocal.service
                ok 'servico systemd (usuario) atualizado e reiniciado'
            else
                systemctl --user enable --now financaslocal.service
                ok 'servico systemd (usuario) registrado e iniciado'
            fi
        else
            aviso "systemctl nao encontrado; unidade escrita em $unidade mas nao habilitada automaticamente."
        fi
        ;;
    osx)
        aviso 'Registro automatico de servico (launchd) nao implementado para macOS.'
        aviso "Para rodar manualmente: $dotnet_exec $dll_destino"
        ;;
    windows)
        # sc.exe e bem especifico sobre espacos: "chave= valor", com um unico
        # espaco apos o "=" e nenhum antes. Por isso binPath= e o valor vao
        # como argumentos separados do bash.
        dotnet_exec_win="$(cygpath -w "$dotnet_exec" 2>/dev/null || echo "$dotnet_exec")"
        dll_destino_win="$(cygpath -w "$dll_destino" 2>/dev/null || echo "$dll_destino")"
        bin_path="\"$dotnet_exec_win\" \"$dll_destino_win\""
        comando_create="sc.exe create $nome_servico binPath= \"$bin_path\" start= auto"
        comando_start="sc.exe start $nome_servico"
        if sc.exe query "$nome_servico" >/dev/null 2>&1; then
            sc.exe stop "$nome_servico" >/dev/null 2>&1 || true
            if ! sc.exe config "$nome_servico" binPath= "$bin_path" start= auto >/dev/null 2>&1; then
                aviso 'Nao foi possivel reconfigurar o servico automaticamente (normalmente requer um prompt elevado). Rode manualmente em um prompt como Administrador:'
                echo "   sc.exe config $nome_servico binPath= \"$bin_path\" start= auto"
                echo "   $comando_start"
            fi
        elif ! sc.exe create "$nome_servico" binPath= "$bin_path" start= auto >/dev/null 2>&1; then
            aviso 'Nao foi possivel criar o servico automaticamente (normalmente requer um prompt elevado). Rode manualmente em um prompt como Administrador:'
            echo "   $comando_create"
            echo "   $comando_start"
        fi
        if sc.exe start "$nome_servico" >/dev/null 2>&1; then
            ok 'servico do Windows registrado e iniciado'
        else
            aviso "Nao foi possivel iniciar o servico automaticamente. Rode manualmente: $comando_start"
        fi
        ;;
esac

# ------------------------------------------------------------------ Teste
etapa 'Testando'
url_teste='https://127.0.0.1:5443/api/auth/estado'
sucesso=0
resposta=""
for _ in 1 2 3 4 5; do
    if resposta="$(curl -k -fsS "$url_teste" 2>/dev/null)"; then
        sucesso=1
        break
    fi
    sleep 2
done
if [[ "$sucesso" -eq 1 ]]; then
    ok "API respondeu: $resposta"
else
    aviso "A API nao respondeu em $url_teste. Verifique os logs do servico."
    ocupante="$(quem_usa_a_porta 5443)"
    [[ -n "$ocupante" ]] && aviso "Quem esta na porta 5443 agora: $ocupante. Se nao for o Financas Local, feche esse programa."
fi

if [[ "$SO" == windows ]] && reg.exe query 'HKLM\SOFTWARE\Mozilla\Mozilla Firefox' >/dev/null 2>&1; then
    aviso 'Firefox instalado: por padrao ele nao usa os certificados do Windows e mostra um aviso no site.'
    aviso 'Para aceitar: em about:config, deixe security.enterprise_roots.enabled como true (ou use o Edge/Chrome).'
fi

echo ''
echo -e "\033[32mPronto! Abra $url_teste no navegador (ajuste a porta se tiver mudado no appsettings.json).\033[0m"
