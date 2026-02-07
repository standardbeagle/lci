@{
    RootModule        = 'LCI.psm1'
    ModuleVersion     = '0.4.0'
    GUID              = 'a1b2c3d4-e5f6-7890-abcd-ef1234567890'
    Author            = 'Standard Beagle'
    CompanyName       = 'Standard Beagle'
    Copyright         = '(c) 2025-2026 Standard Beagle. All rights reserved.'
    Description       = 'PowerShell wrapper for Lightning Code Index (lci). Provides structured object output with navigation properties that eliminate the need for tracking internal IDs.'

    PowerShellVersion = '5.1'

    FunctionsToExport = @(
        'Search-LciCode'
        'Find-LciText'
        'Get-LciDefinition'
        'Get-LciReference'
        'Get-LciCallTree'
        'Get-LciStatus'
        'Get-LciSymbol'
        'Get-LciSymbolDetail'
        'Get-LciFileOutline'
        'Invoke-LciGitAnalysis'
        'Start-LciServer'
        'Stop-LciServer'
        'Set-LciExecutable'
        'Set-LciRoot'
        'Set-LciConfig'
    )

    AliasesToExport = @(
        'lcis'      # Search-LciCode
        'lcig'      # Find-LciText
        'lcid'      # Get-LciDefinition
        'lcir'      # Get-LciReference
        'lcit'      # Get-LciCallTree
        'lcisym'    # Get-LciSymbol
        'lcii'      # Get-LciSymbolDetail
        'lcibr'     # Get-LciFileOutline
    )

    CmdletsToExport   = @()
    VariablesToExport  = @()

    PrivateData = @{
        PSData = @{
            Tags         = @('code-index', 'search', 'symbols', 'lci', 'code-navigation')
            ProjectUri   = 'https://github.com/standardbeagle/lci'
            LicenseUri   = 'https://github.com/standardbeagle/lci/blob/main/LICENSE'
        }
    }
}
