using Microsoft.Win32;

namespace Financas.Gerenciador;

/// <summary>Preferências do gerenciador, por usuário (HKCU): nada disso precisa de administrador.</summary>
internal static class Preferencias
{
    private const string Chave = @"Software\FinancasLocal\Gerenciador";
    private const string ChaveIniciar = @"Software\Microsoft\Windows\CurrentVersion\Run";
    private const string NomeIniciar = "FinancasLocalGerenciador";

    /// <summary>Procura versão nova ao abrir (padrão: sim). Só avisa; instalar é sempre um clique seu.</summary>
    public static bool ProcurarAoAbrir
    {
        get => Registry.CurrentUser.OpenSubKey(Chave)?.GetValue("ProcurarAoAbrir") is not int valor || valor != 0;
        set
        {
            using var chave = Registry.CurrentUser.CreateSubKey(Chave);
            chave.SetValue("ProcurarAoAbrir", value ? 1 : 0, RegistryValueKind.DWord);
        }
    }

    /// <summary>Abre na área de notificação quando o Windows inicia (padrão: não).</summary>
    public static bool IniciarComWindows
    {
        get => Registry.CurrentUser.OpenSubKey(ChaveIniciar)?.GetValue(NomeIniciar) is not null;
        set
        {
            using var chave = Registry.CurrentUser.CreateSubKey(ChaveIniciar);
            if (value) chave.SetValue(NomeIniciar, $"{AcoesElevadas.Aspas(Environment.ProcessPath!)} --bandeja");
            else chave.DeleteValue(NomeIniciar, throwOnMissingValue: false);
        }
    }
}
