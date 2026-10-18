<#
    Desenha as imagens do instalador .msi (WixUI) com a identidade do site: o degrade da marca (#b9b6ff -> #9fd2f6),
    o cofrinho (icone-512.png do site) e o nome. Rode de novo so se a marca mudar; as imagens ficam no repositorio.

    .\instalador\imagens\gerar-imagens.ps1

    - dialogo.bmp (493x312): fundo das telas de boas-vindas e de conclusao. A faixa da marca fica a esquerda; o texto do
      WixUI comeca em ~180 px, por isso o resto e branco.
    - faixa.bmp (493x58): faixa do topo das outras telas. O titulo e a descricao do WixUI vao ate ~406 px; o cofrinho
      fica a direita, sobre o degrade.
    Windows PowerShell 5.1 (System.Drawing) e sem acentos no arquivo.
#>
$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Drawing

$Raiz = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$Icone = Join-Path $Raiz "src\frontend\public\icone-512.png"
$Nome = "Finan" + [char]0x00E7 + "as Local"

$inicio = [System.Drawing.ColorTranslator]::FromHtml("#b9b6ff")
$fim = [System.Drawing.ColorTranslator]::FromHtml("#9fd2f6")
$textoMarca = [System.Drawing.ColorTranslator]::FromHtml("#23214a")
$cofrinho = [System.Drawing.Image]::FromFile($Icone)

function Novo-Desenho([int]$largura, [int]$altura) {
    $imagem = New-Object System.Drawing.Bitmap($largura, $altura, [System.Drawing.Imaging.PixelFormat]::Format24bppRgb)
    $g = [System.Drawing.Graphics]::FromImage($imagem)
    $g.SmoothingMode = "AntiAlias"
    $g.InterpolationMode = "HighQualityBicubic"
    $g.TextRenderingHint = "ClearTypeGridFit"
    $g.Clear([System.Drawing.Color]::White)
    return @($imagem, $g)
}

function Retangulo-Arredondado([single]$x, [single]$y, [single]$l, [single]$a, [single]$r) {
    $d = $r * 2
    $caminho = New-Object System.Drawing.Drawing2D.GraphicsPath
    $caminho.AddArc($x, $y, $d, $d, 180, 90)
    $caminho.AddArc($x + $l - $d, $y, $d, $d, 270, 90)
    $caminho.AddArc($x + $l - $d, $y + $a - $d, $d, $d, 0, 90)
    $caminho.AddArc($x, $y + $a - $d, $d, $d, 90, 90)
    $caminho.CloseFigure()
    return $caminho
}

function Cofrinho($g, [single]$x, [single]$y, [single]$lado) {
    $moldura = Retangulo-Arredondado ($x - 6) ($y - 6) ($lado + 12) ($lado + 12) 14
    $branco = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::FromArgb(160, 255, 255, 255))
    $g.FillPath($branco, $moldura)
    $g.DrawImage($cofrinho, [single]$x, [single]$y, [single]$lado, [single]$lado)
}

# Telas de boas-vindas e de conclusao.
$par = Novo-Desenho 493 312
$dialogo = $par[0]; $g = $par[1]
$faixa = New-Object System.Drawing.Rectangle(0, 0, 164, 312)
$degrade = New-Object System.Drawing.Drawing2D.LinearGradientBrush($faixa, $inicio, $fim, 60)
$g.FillRectangle($degrade, $faixa)
Cofrinho $g 50 84 64
$fonteNome = New-Object System.Drawing.Font("Segoe UI Semibold", 13)
$fonteSub = New-Object System.Drawing.Font("Segoe UI", 9)
$centro = New-Object System.Drawing.StringFormat
$centro.Alignment = "Center"
$pincel = New-Object System.Drawing.SolidBrush($textoMarca)
$g.DrawString($Nome, $fonteNome, $pincel, (New-Object System.Drawing.RectangleF(0, 172, 164, 30)), $centro)
$g.DrawString("local e privado", $fonteSub, $pincel, (New-Object System.Drawing.RectangleF(0, 200, 164, 20)), $centro)
$g.Dispose()
$dialogo.Save((Join-Path $PSScriptRoot "dialogo.bmp"), [System.Drawing.Imaging.ImageFormat]::Bmp)

# Faixa do topo das outras telas: branca onde fica o texto, com o degrade e o cofrinho a direita.
$par = Novo-Desenho 493 58
$banner = $par[0]; $g = $par[1]
$area = New-Object System.Drawing.Rectangle(400, 0, 93, 58)
$suave = New-Object System.Drawing.Drawing2D.LinearGradientBrush((New-Object System.Drawing.Rectangle(399, 0, 94, 58)),
    [System.Drawing.Color]::White, $inicio, 0)
$suave.InterpolationColors = New-Object System.Drawing.Drawing2D.ColorBlend -Property @{
    Colors = @([System.Drawing.Color]::White, $inicio, $fim); Positions = @([single]0, [single]0.55, [single]1)
}
$g.FillRectangle($suave, $area)
Cofrinho $g 445 13 32
$linha = New-Object System.Drawing.Pen([System.Drawing.ColorTranslator]::FromHtml("#e2dfee"))
$g.DrawLine($linha, 0, 57, 493, 57)
$g.Dispose()
$banner.Save((Join-Path $PSScriptRoot "faixa.bmp"), [System.Drawing.Imaging.ImageFormat]::Bmp)

Write-Host "Pronto: dialogo.bmp e faixa.bmp em $PSScriptRoot"
