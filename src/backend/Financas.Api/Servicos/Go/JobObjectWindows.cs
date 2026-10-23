using System.ComponentModel;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Runtime.Versioning;
using Microsoft.Win32.SafeHandles;

namespace Financas.Api.Servicos.Go;

/// <summary>
/// No Windows, o Go entra num Job Object com "matar ao fechar" (JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE): o único handle
/// do job é deste processo, e quando o .NET termina, de qualquer jeito (até morto pelo Gerenciador de Tarefas), o
/// Windows fecha o handle e encerra o Go junto. Sem isso, o Go ficaria órfão segurando o banco e o executável.
/// O Go também para sozinho quando a entrada padrão fecha; o job cobre o caso de ele estar travado.
/// https://learn.microsoft.com/windows/win32/procthread/job-objects
/// </summary>
[SupportedOSPlatform("windows")]
internal static class JobObjectWindows
{
    private const int JobObjectExtendedLimitInformation = 9;
    private const uint JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE = 0x2000;

    private static readonly Lazy<SafeFileHandle> Job = new(Criar);

    /// <summary>Põe o processo no job. Lança <see cref="Win32Exception"/> se o Windows recusar.</summary>
    public static void Atribuir(Process processo)
    {
        if (!AssignProcessToJobObject(Job.Value, processo.Handle))
            throw new Win32Exception(Marshal.GetLastWin32Error(), "Não deu para pôr o Go no Job Object.");
    }

    private static SafeFileHandle Criar()
    {
        var job = CreateJobObjectW(IntPtr.Zero, null);
        if (job.IsInvalid) throw new Win32Exception(Marshal.GetLastWin32Error(), "Não deu para criar o Job Object.");
        var info = new JOBOBJECT_EXTENDED_LIMIT_INFORMATION
        {
            BasicLimitInformation = new JOBOBJECT_BASIC_LIMIT_INFORMATION { LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE }
        };
        if (!SetInformationJobObject(job, JobObjectExtendedLimitInformation, ref info,
                (uint)Marshal.SizeOf<JOBOBJECT_EXTENDED_LIMIT_INFORMATION>()))
        {
            var erro = Marshal.GetLastWin32Error();
            job.Dispose();
            throw new Win32Exception(erro, "Não deu para configurar o Job Object.");
        }
        // O handle fica aberto até o processo terminar: é o fechamento dele que encerra o Go.
        return job;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct JOBOBJECT_BASIC_LIMIT_INFORMATION
    {
        public long PerProcessUserTimeLimit;
        public long PerJobUserTimeLimit;
        public uint LimitFlags;
        public UIntPtr MinimumWorkingSetSize;
        public UIntPtr MaximumWorkingSetSize;
        public uint ActiveProcessLimit;
        public UIntPtr Affinity;
        public uint PriorityClass;
        public uint SchedulingClass;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct IO_COUNTERS
    {
        public ulong ReadOperationCount;
        public ulong WriteOperationCount;
        public ulong OtherOperationCount;
        public ulong ReadTransferCount;
        public ulong WriteTransferCount;
        public ulong OtherTransferCount;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct JOBOBJECT_EXTENDED_LIMIT_INFORMATION
    {
        public JOBOBJECT_BASIC_LIMIT_INFORMATION BasicLimitInformation;
        public IO_COUNTERS IoInfo;
        public UIntPtr ProcessMemoryLimit;
        public UIntPtr JobMemoryLimit;
        public UIntPtr PeakProcessMemoryUsed;
        public UIntPtr PeakJobMemoryUsed;
    }

    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern SafeFileHandle CreateJobObjectW(IntPtr atributos, string? nome);

    [DllImport("kernel32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool SetInformationJobObject(SafeFileHandle job, int classe,
        ref JOBOBJECT_EXTENDED_LIMIT_INFORMATION info, uint tamanho);

    [DllImport("kernel32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool AssignProcessToJobObject(SafeFileHandle job, IntPtr processo);
}
