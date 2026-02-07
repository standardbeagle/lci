#Requires -Version 5.1
<#
.SYNOPSIS
    PowerShell wrapper module for Lightning Code Index (lci).

.DESCRIPTION
    Provides structured PowerShell cmdlets around the lci CLI tool.
    All results are rich objects with navigation methods that eliminate
    the need for tracking or passing internal IDs.

    Navigation works two ways:
      1. Pipeline:  Search-LciCode "Foo" | Get-LciDefinition
      2. Methods:   (Search-LciCode "Foo")[0].Inspect()

.NOTES
    Install: Import-Module ./powershell/LCI/LCI.psd1
    Requires: lci binary on PATH (or set with Set-LciExecutable)
#>

# ---------------------------------------------------------------------------
# Module state
# ---------------------------------------------------------------------------
$script:LciExe = 'lci'
$script:LciRoot = $null          # --root override (null = use lci default)
$script:LciConfig = $null        # --config override

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
function Set-LciExecutable {
    <#
    .SYNOPSIS Sets the path to the lci binary.
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$Path
    )
    $script:LciExe = $Path
}

function Set-LciRoot {
    <#
    .SYNOPSIS Sets the project root for all subsequent lci commands.
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$Path
    )
    $script:LciRoot = $Path
}

function Set-LciConfig {
    <#
    .SYNOPSIS Sets the config file path for all subsequent lci commands.
    #>
    [CmdletBinding()]
    param([string]$Path)
    $script:LciConfig = $Path
}

# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------
function Build-LciArgs {
    <#
    .SYNOPSIS Builds the global argument prefix (--root, --config).
    #>
    $args_ = @()
    if ($script:LciRoot)   { $args_ += '--root';   $args_ += $script:LciRoot }
    if ($script:LciConfig) { $args_ += '--config'; $args_ += $script:LciConfig }
    return $args_
}

function Invoke-Lci {
    <#
    .SYNOPSIS Runs lci with arguments and returns parsed JSON or raw text.
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string[]]$Arguments,
        [switch]$RawText
    )

    $allArgs = @(Build-LciArgs) + $Arguments

    # Use temp files to cleanly separate stdout/stderr
    $stdoutFile = [System.IO.Path]::GetTempFileName()
    $stderrFile = [System.IO.Path]::GetTempFileName()

    try {
        $psi = [System.Diagnostics.ProcessStartInfo]::new()
        $psi.FileName = $script:LciExe
        foreach ($a in $allArgs) { $psi.ArgumentList.Add($a) }
        $psi.RedirectStandardOutput = $true
        $psi.RedirectStandardError  = $true
        $psi.UseShellExecute = $false
        $psi.CreateNoWindow  = $true

        $proc = [System.Diagnostics.Process]::Start($psi)

        # Read stdout and stderr concurrently to avoid deadlocks
        $stdoutTask = $proc.StandardOutput.ReadToEndAsync()
        $stderrTask = $proc.StandardError.ReadToEndAsync()

        $proc.WaitForExit(60000) | Out-Null  # 60s timeout

        $stdout = $stdoutTask.GetAwaiter().GetResult()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        $exitCode = $proc.ExitCode
    } catch {
        throw "Failed to execute lci: $_"
    } finally {
        if ($proc) { $proc.Dispose() }
        Remove-Item -Path $stdoutFile, $stderrFile -Force -ErrorAction SilentlyContinue
    }

    if ($exitCode -ne 0 -and -not $stdout.Trim()) {
        $msg = if ($stderr.Trim()) { $stderr.Trim() } else { "lci exited with code $exitCode" }
        throw $msg
    }

    if ($RawText) { return $stdout }

    if (-not $stdout.Trim()) { return $null }

    # Strip any non-JSON prefix lines (e.g. DEBUG: lines from search)
    $lines = $stdout -split "`n"
    $jsonStartIdx = -1
    for ($i = 0; $i -lt $lines.Count; $i++) {
        if ($lines[$i] -match '^\s*[\[{"]') {
            $jsonStartIdx = $i
            break
        }
    }

    if ($jsonStartIdx -lt 0) { return $null }

    $json = ($lines[$jsonStartIdx..($lines.Count - 1)]) -join "`n"
    if (-not $json.Trim()) { return $null }

    try {
        return $json | ConvertFrom-Json
    } catch {
        throw "Failed to parse lci JSON output: $_`nFirst 200 chars: $($json.Substring(0, [Math]::Min(200, $json.Length)))"
    }
}

function Add-NavigationMethods {
    <#
    .SYNOPSIS Attaches navigation ScriptMethods to an object based on its type name.
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [PSObject]$Object
    )

    $typeName = $Object.PSTypeNames[0]

    # All types with a symbol name get Inspect / Definition / References / Tree
    $symbolName = $null
    $filePath   = $null

    switch -Wildcard ($typeName) {
        'LCI.SearchResult' {
            $symbolName = $Object.BlockName
            if (-not $symbolName) { $symbolName = $Object.Match }
            $filePath = $Object.Path
        }
        'LCI.GrepResult' {
            $symbolName = $Object.Match
            $filePath = $Object.Path
        }
        'LCI.Definition' {
            $symbolName = $Object.Name
            $filePath = $Object.FilePath
        }
        'LCI.Reference' {
            $symbolName = $Object.Match
            $filePath = $Object.FilePath
        }
        'LCI.Symbol' {
            $symbolName = $Object.Name
            $filePath = $Object.File
        }
        'LCI.SymbolDetail' {
            $symbolName = $Object.Name
            $filePath = $Object.File
        }
        'LCI.FileSymbol' {
            $symbolName = $Object.Name
            $filePath = $Object.File
        }
    }

    if ($symbolName) {
        $Object | Add-Member -MemberType ScriptMethod -Name 'Inspect' -Force -Value {
            Get-LciSymbolDetail -Name $this._SymbolName -File $this._FilePath
        }.GetNewClosure()
        $Object | Add-Member -MemberType NoteProperty -Name '_SymbolName' -Value $symbolName -Force
        $Object | Add-Member -MemberType NoteProperty -Name '_FilePath'   -Value $filePath   -Force

        $Object | Add-Member -MemberType ScriptMethod -Name 'Definition' -Force -Value {
            Get-LciDefinition -Name $this._SymbolName
        }.GetNewClosure()

        $Object | Add-Member -MemberType ScriptMethod -Name 'References' -Force -Value {
            Get-LciReference -Name $this._SymbolName
        }.GetNewClosure()

        $Object | Add-Member -MemberType ScriptMethod -Name 'Tree' -Force -Value {
            param([int]$MaxDepth = 5)
            Get-LciCallTree -Name $this._SymbolName -MaxDepth $MaxDepth
        }.GetNewClosure()
    }

    if ($filePath) {
        $Object | Add-Member -MemberType ScriptMethod -Name 'Browse' -Force -Value {
            Get-LciFileOutline -Path $this._FilePath
        }.GetNewClosure()
    }

    # SymbolDetail gets caller/callee navigation
    if ($typeName -eq 'LCI.SymbolDetail') {
        $Object | Add-Member -MemberType ScriptMethod -Name 'GetCallers' -Force -Value {
            $this.Callers | ForEach-Object { Get-LciSymbolDetail -Name $_ }
        }.GetNewClosure()

        $Object | Add-Member -MemberType ScriptMethod -Name 'GetCallees' -Force -Value {
            $this.Callees | ForEach-Object { Get-LciSymbolDetail -Name $_ }
        }.GetNewClosure()

        if ($Object.TypeHierarchy) {
            $Object | Add-Member -MemberType ScriptMethod -Name 'GetImplementors' -Force -Value {
                $this.TypeHierarchy.ImplementedBy | ForEach-Object { Get-LciSymbolDetail -Name $_ }
            }.GetNewClosure()

            $Object | Add-Member -MemberType ScriptMethod -Name 'GetInterfaces' -Force -Value {
                $this.TypeHierarchy.Implements | ForEach-Object { Get-LciSymbolDetail -Name $_ }
            }.GetNewClosure()
        }
    }

}

function New-LciObject {
    <#
    .SYNOPSIS Creates a typed PSCustomObject with the given type name and adds navigation.
    #>
    param(
        [string]$TypeName,
        [hashtable]$Properties
    )

    $obj = [PSCustomObject]$Properties
    $obj.PSTypeNames.Insert(0, $TypeName)
    Add-NavigationMethods -Object $obj | Out-Null
    $obj
}

# ---------------------------------------------------------------------------
# Public cmdlets
# ---------------------------------------------------------------------------

function Search-LciCode {
    <#
    .SYNOPSIS
        Search for a pattern in the indexed codebase (full semantic analysis).
    .DESCRIPTION
        Wraps 'lci search --json'. Returns LCI.SearchResult objects with
        navigation methods: .Inspect(), .Definition(), .References(),
        .Tree(), .Browse().
    .EXAMPLE
        Search-LciCode "HandleRequest"
    .EXAMPLE
        Search-LciCode "TODO" -CommentsOnly | Group-Object Path
    .EXAMPLE
        Search-LciCode "error" -CaseInsensitive | Get-LciDefinition
    #>
    [CmdletBinding()]
    [OutputType('LCI.SearchResult')]
    param(
        [Parameter(Mandatory, Position = 0)]
        [string]$Pattern,

        [int]$MaxLines = 0,
        [switch]$CaseInsensitive,
        [string]$Exclude,
        [string]$Include,
        [switch]$CommentsOnly,
        [switch]$CodeOnly,
        [switch]$StringsOnly,
        [switch]$WordRegexp,
        [switch]$Regex,
        [switch]$InvertMatch,
        [switch]$FilesOnly,
        [switch]$Count,
        [int]$MaxCount = 0,
        [switch]$Compact,
        [ValidateSet('relevance','proximity','similarity')]
        [string]$RankBy,
        [string]$ContextFilter
    )

    $args_ = @('search', '--json', $Pattern)
    if ($MaxLines -gt 0)   { $args_ += '--max-lines';     $args_ += $MaxLines }
    if ($CaseInsensitive)  { $args_ += '--case-insensitive' }
    if ($Exclude)          { $args_ += '--exclude';        $args_ += $Exclude }
    if ($Include)          { $args_ += '--include';        $args_ += $Include }
    if ($CommentsOnly)     { $args_ += '--comments-only' }
    if ($CodeOnly)         { $args_ += '--code-only' }
    if ($StringsOnly)      { $args_ += '--strings-only' }
    if ($WordRegexp)       { $args_ += '--word-regexp' }
    if ($Regex)            { $args_ += '--regex' }
    if ($InvertMatch)      { $args_ += '--invert-match' }
    if ($FilesOnly)        { $args_ += '--files-with-matches' }
    if ($Count)            { $args_ += '--count' }
    if ($MaxCount -gt 0)   { $args_ += '--max-count';     $args_ += $MaxCount }
    if ($Compact)          { $args_ += '--compact-search' }
    if ($RankBy)           { $args_ += '--rank-by';        $args_ += $RankBy }
    if ($ContextFilter)    { $args_ += '--context-filter'; $args_ += $ContextFilter }

    $data = Invoke-Lci -Arguments $args_
    if (-not $data) { return }

    # Attach query metadata to each result
    foreach ($r in $data.results) {
        $isStandard = $null -ne $r.result   # StandardResult has nested .result

        $grep = if ($isStandard) { $r.result } else { $r }

        New-LciObject -TypeName 'LCI.SearchResult' -Properties @{
            Query       = $data.query
            TimeMs      = $data.time_ms
            TotalCount  = $data.count
            Mode        = $data.mode
            Path        = $grep.path
            Line        = $grep.line
            Column      = $grep.column
            Match       = $grep.match
            Score       = $grep.score
            Context     = $grep.context.lines
            StartLine   = $grep.context.start_line
            EndLine     = $grep.context.end_line
            BlockType   = $grep.context.block_type
            BlockName   = $grep.context.block_name
            IsComplete  = $grep.context.is_complete
            MatchCount  = $grep.context.match_count
            ObjectID    = if ($isStandard) { $r.object_id } else { $null }
        }
    }
}

function Find-LciText {
    <#
    .SYNOPSIS
        Ultra-fast text search (grep mode, 40% faster, 75% less memory).
    .DESCRIPTION
        Wraps 'lci grep --json'. Returns LCI.GrepResult objects.
    .EXAMPLE
        Find-LciText "TODO" -CaseInsensitive
    .EXAMPLE
        Find-LciText "func.*Handler" -Regex -ExcludeTests
    #>
    [CmdletBinding()]
    [OutputType('LCI.GrepResult')]
    param(
        [Parameter(Mandatory, Position = 0)]
        [string]$Pattern,

        [int]$MaxResults = 500,
        [int]$ContextLines = 3,
        [switch]$CaseInsensitive,
        [string]$Exclude,
        [string]$Include,
        [switch]$ExcludeTests,
        [switch]$ExcludeComments,
        [switch]$Regex
    )

    $args_ = @('grep', '--json', $Pattern)
    if ($MaxResults -ne 500) { $args_ += '--max-results'; $args_ += $MaxResults }
    if ($ContextLines -ne 3) { $args_ += '--context';     $args_ += $ContextLines }
    if ($CaseInsensitive)    { $args_ += '--case-insensitive' }
    if ($Exclude)            { $args_ += '--exclude';      $args_ += $Exclude }
    if ($Include)            { $args_ += '--include';      $args_ += $Include }
    if ($ExcludeTests)       { $args_ += '--exclude-tests' }
    if ($ExcludeComments)    { $args_ += '--exclude-comments' }
    if ($Regex)              { $args_ += '--regex' }

    $data = Invoke-Lci -Arguments $args_
    if (-not $data) { return }

    foreach ($r in $data.results) {
        New-LciObject -TypeName 'LCI.GrepResult' -Properties @{
            Query      = $data.query
            TimeMs     = $data.time_ms
            TotalCount = $data.count
            Path       = $r.path
            Line       = $r.line
            Column     = $r.column
            Match      = $r.match
            Score      = $r.score
            Context    = $r.context.lines
            StartLine  = $r.context.start_line
            EndLine    = $r.context.end_line
            BlockType  = $r.context.block_type
            BlockName  = $r.context.block_name
        }
    }
}

function Get-LciDefinition {
    <#
    .SYNOPSIS
        Find where a symbol is defined.
    .DESCRIPTION
        Wraps 'lci def'. Returns LCI.Definition objects with navigation.
        Accepts pipeline input from other LCI cmdlets.
    .EXAMPLE
        Get-LciDefinition "Server"
    .EXAMPLE
        Get-LciSymbol -Kind struct | Get-LciDefinition
    #>
    [CmdletBinding()]
    [OutputType('LCI.Definition')]
    param(
        [Parameter(Mandatory, Position = 0, ValueFromPipelineByPropertyName)]
        [Alias('_SymbolName','Match','BlockName')]
        [string]$Name
    )

    process {
        # def doesn't have --json, so we use inspect --json as a richer alternative
        # and return the definition location from it
        $args_ = @('inspect', '--json', '--include', 'signature,doc', $Name)

        try {
            $data = Invoke-Lci -Arguments $args_
        } catch {
            # Fallback: parse text output from def
            $text = Invoke-Lci -Arguments @('def', $Name) -RawText
            if (-not $text) { return }

            foreach ($line in ($text -split "`n")) {
                if ($line -match '^(.+?):(\d+):\s*(.+)$') {
                    $fp   = $Matches[1]
                    $ln   = [int]$Matches[2]
                    $rest = $Matches[3]

                    # Try to parse "type name" or full signature
                    $symType = ''; $symName = $Name; $sig = $rest
                    if ($rest -match '^(function|method|struct|class|interface|type|enum|variable|constant)\s+(.+)$') {
                        $symType = $Matches[1]
                        $sig     = $Matches[2]
                    }

                    New-LciObject -TypeName 'LCI.Definition' -Properties @{
                        Name       = $symName
                        Type       = $symType
                        FilePath   = $fp
                        Line       = $ln
                        Column     = 0
                        Signature  = $sig
                        DocComment = ''
                    }
                }
            }
            return
        }

        if (-not $data -or -not $data.symbols) { return }

        foreach ($sym in $data.symbols) {
            New-LciObject -TypeName 'LCI.Definition' -Properties @{
                Name       = $sym.name
                Type       = $sym.type
                FilePath   = $sym.file
                Line       = $sym.line
                Column     = 0
                Signature  = $sym.signature
                DocComment = $sym.doc_comment
            }
        }
    }
}

function Get-LciReference {
    <#
    .SYNOPSIS
        Find all references (usages) of a symbol.
    .DESCRIPTION
        Wraps 'lci refs'. Returns LCI.Reference objects.
        Accepts pipeline input from other LCI cmdlets.
    .EXAMPLE
        Get-LciReference "HandleRequest"
    .EXAMPLE
        Get-LciDefinition "Server" | Get-LciReference
    #>
    [CmdletBinding()]
    [OutputType('LCI.Reference')]
    param(
        [Parameter(Mandatory, Position = 0, ValueFromPipelineByPropertyName)]
        [Alias('_SymbolName','Match','BlockName')]
        [string]$Name
    )

    process {
        $text = Invoke-Lci -Arguments @('refs', $Name) -RawText
        if (-not $text) { return }

        foreach ($line in ($text -split "`n")) {
            if ($line -match '^(.+?):(\d+):\s*(.*)$') {
                New-LciObject -TypeName 'LCI.Reference' -Properties @{
                    SymbolName = $Name
                    FilePath   = $Matches[1]
                    Line       = [int]$Matches[2]
                    Context    = $Matches[3]
                    Match      = $Name
                }
            }
        }
    }
}

function Get-LciCallTree {
    <#
    .SYNOPSIS
        Display the function call hierarchy tree.
    .DESCRIPTION
        Wraps 'lci tree --json'. Returns an LCI.CallTree root with
        recursively nested Children nodes (LCI.TreeNode).
        Accepts pipeline input from other LCI cmdlets.
    .EXAMPLE
        Get-LciCallTree "main" -MaxDepth 3
    .EXAMPLE
        Get-LciSymbol -Kind func -Name "Handle" | Get-LciCallTree
    #>
    [CmdletBinding()]
    [OutputType('LCI.CallTree')]
    param(
        [Parameter(Mandatory, Position = 0, ValueFromPipelineByPropertyName)]
        [Alias('_SymbolName','Match','BlockName')]
        [string]$Name,

        [int]$MaxDepth = 5,
        [switch]$Compact,
        [switch]$AgentMode,
        [switch]$Metrics,
        [string]$Exclude
    )

    process {
        $args_ = @('tree', '--json', $Name, '--max-depth', $MaxDepth)
        if ($Compact)   { $args_ += '--compact' }
        if ($AgentMode) { $args_ += '--agent' }
        if ($Metrics)   { $args_ += '--metrics' }
        if ($Exclude)   { $args_ += '--exclude'; $args_ += $Exclude }

        $data = Invoke-Lci -Arguments $args_
        if (-not $data -or -not $data.tree) { return }

        $tree = $data.tree

        # Recursively convert tree nodes
        function Convert-TreeNode {
            param($Node, [int]$Depth = 0)
            if (-not $Node) { return $null }

            $children = @()
            if ($Node.children) {
                foreach ($child in $Node.children) {
                    $children += Convert-TreeNode -Node $child -Depth ($Depth + 1)
                }
            }

            New-LciObject -TypeName 'LCI.TreeNode' -Properties @{
                Name            = $Node.name
                FilePath        = $Node.file_path
                Line            = $Node.line
                Depth           = $Node.depth
                NodeType        = $Node.node_type
                Annotations     = $Node.annotations
                Children        = $children
                ChildCount      = $children.Count
                EditRiskScore   = $Node.edit_risk_score
                StabilityTags   = $Node.stability_tags
                DependencyCount = $Node.dependency_count
                DependentCount  = $Node.dependent_count
                ImpactRadius    = $Node.impact_radius
                SafetyNotes     = $Node.safety_notes
            }
        }

        $rootNode = Convert-TreeNode -Node $tree.root

        $result = New-LciObject -TypeName 'LCI.CallTree' -Properties @{
            FunctionName = $data.function
            TimeMs       = $data.time_ms
            RootFunction = $tree.root_function
            MaxDepth     = $tree.max_depth
            TotalNodes   = $tree.total_nodes
            Root         = $rootNode
        }

        # Add a Flatten method to get all nodes as a flat list
        $result | Add-Member -MemberType ScriptMethod -Name 'Flatten' -Force -Value {
            function Recurse($node) {
                $node
                if ($node.Children) {
                    foreach ($c in $node.Children) { Recurse $c }
                }
            }
            Recurse $this.Root
        }

        # Add a ToString rendering
        $result | Add-Member -MemberType ScriptMethod -Name 'ToText' -Force -Value {
            function RenderNode($node, $indent) {
                $line = if ($node.Line) { ":$($node.Line)" } else { '' }
                $file = if ($node.FilePath) { " ($($node.FilePath)$line)" } else { '' }
                "$indent$($node.Name)$file"
                if ($node.Children) {
                    foreach ($c in $node.Children) {
                        RenderNode $c "$indent  "
                    }
                }
            }
            (RenderNode $this.Root '') -join "`n"
        }

        $result
    }
}

function Get-LciStatus {
    <#
    .SYNOPSIS
        Show index server status and statistics.
    .DESCRIPTION
        Wraps 'lci status --json'. Returns an LCI.Status object.
    .EXAMPLE
        Get-LciStatus
    .EXAMPLE
        (Get-LciStatus).SymbolCount
    #>
    [CmdletBinding()]
    [OutputType('LCI.Status')]
    param()

    $data = Invoke-Lci -Arguments @('status', '--json')
    if (-not $data) { return }

    [PSCustomObject]@{
        PSTypeName      = 'LCI.Status'
        Timestamp       = if ($data.timestamp) { [DateTime]$data.timestamp } else { [DateTime]::Now }
        Ready           = [bool]$data.ready
        FileCount       = [int]$data.file_count
        SymbolCount     = [int]$data.symbol_count
        IndexSizeBytes  = [long]$data.index_size_bytes
        BuildDurationMs = [long]$data.build_duration_ms
        MemoryAllocMB   = [double]$data.memory_alloc_mb
        MemoryTotalMB   = [double]$data.memory_total_mb
        MemoryHeapMB    = [double]$data.memory_heap_mb
        NumGoroutines   = [int]$data.num_goroutines
        UptimeSeconds   = [double]$data.uptime_seconds
        SearchCount     = [long]$data.search_count
        AvgSearchTimeMs = [double]$data.avg_search_time_ms
    }
}

function Get-LciSymbol {
    <#
    .SYNOPSIS
        List and filter symbols in the index.
    .DESCRIPTION
        Wraps 'lci symbols --json'. Returns LCI.Symbol objects with
        navigation methods. Supports rich filtering by kind, name,
        complexity, receiver type, and more.
    .EXAMPLE
        Get-LciSymbol -Kind func -Exported
    .EXAMPLE
        Get-LciSymbol -Kind method -Receiver "Server" | Get-LciSymbolDetail
    .EXAMPLE
        Get-LciSymbol -Kind func -MinComplexity 10 -Sort complexity
    #>
    [CmdletBinding()]
    [OutputType('LCI.Symbol')]
    param(
        [ValidateSet('all','func','type','struct','interface','method',
                     'class','enum','variable','constant')]
        [string]$Kind = 'all',

        [switch]$Exported,
        [string]$File,
        [string]$Name,
        [string]$Receiver,
        [int]$MinComplexity,
        [int]$MaxComplexity,

        [ValidateSet('name','complexity','refs','line','params')]
        [string]$Sort = 'name',

        [int]$Max = 50
    )

    $args_ = @('symbols', '--json', '--kind', $Kind, '--sort', $Sort, '--max', $Max)
    if ($Exported)                      { $args_ += '--exported' }
    if ($File)                          { $args_ += '--file';           $args_ += $File }
    if ($Name)                          { $args_ += '--name';           $args_ += $Name }
    if ($Receiver)                      { $args_ += '--receiver';       $args_ += $Receiver }
    if ($PSBoundParameters.ContainsKey('MinComplexity')) { $args_ += '--min-complexity'; $args_ += $MinComplexity }
    if ($PSBoundParameters.ContainsKey('MaxComplexity')) { $args_ += '--max-complexity'; $args_ += $MaxComplexity }

    $data = Invoke-Lci -Arguments $args_
    if (-not $data -or -not $data.symbols) { return }

    foreach ($sym in $data.symbols) {
        $obj = New-LciObject -TypeName 'LCI.Symbol' -Properties @{
            Name           = $sym.name
            Type           = $sym.type
            File           = $sym.file
            Line           = $sym.line
            ObjectID       = $sym.object_id
            IsExported     = [bool]$sym.is_exported
            Signature      = $sym.signature
            Complexity     = [int]$sym.complexity
            ParameterCount = [int]$sym.parameter_count
            ReceiverType   = $sym.receiver_type
            IncomingRefs   = [int]$sym.incoming_refs
            OutgoingRefs   = [int]$sym.outgoing_refs
            Callers        = $sym.callers
            Callees        = $sym.callees
        }
        $obj
    }

    if ($data.has_more) {
        Write-Warning "Showing $($data.showing) of $($data.total) symbols. Use -Max to see more."
    }
}

function Get-LciSymbolDetail {
    <#
    .SYNOPSIS
        Deep-inspect a symbol: signature, doc, callers, callees, type hierarchy, scope chain.
    .DESCRIPTION
        Wraps 'lci inspect --json'. Returns LCI.SymbolDetail objects with
        navigation methods including GetCallers(), GetCallees(),
        GetImplementors(), and GetInterfaces().
        Accepts pipeline input from other LCI cmdlets.
    .EXAMPLE
        Get-LciSymbolDetail "Server"
    .EXAMPLE
        Get-LciSymbol -Kind struct | Get-LciSymbolDetail
    .EXAMPLE
        (Get-LciSymbolDetail "handleSearch").GetCallers()
    #>
    [CmdletBinding()]
    [OutputType('LCI.SymbolDetail')]
    param(
        [Parameter(Mandatory, Position = 0, ValueFromPipelineByPropertyName)]
        [Alias('_SymbolName','Match','BlockName')]
        [string]$Name,

        [string]$File,
        [string]$Type,

        [ValidateSet('all','signature','doc','callers','callees',
                     'type_hierarchy','scope','refs','annotations','flags')]
        [string]$Include = 'all'
    )

    process {
        $args_ = @('inspect', '--json', $Name)
        if ($File)                { $args_ += '--file';    $args_ += $File }
        if ($Type)                { $args_ += '--type';    $args_ += $Type }
        if ($Include -ne 'all')   { $args_ += '--include'; $args_ += $Include }

        $data = Invoke-Lci -Arguments $args_
        if (-not $data -or -not $data.symbols) { return }

        foreach ($sym in $data.symbols) {
            New-LciObject -TypeName 'LCI.SymbolDetail' -Properties @{
                Name           = $sym.name
                ObjectID       = $sym.object_id
                Type           = $sym.type
                File           = $sym.file
                Line           = $sym.line
                IsExported     = [bool]$sym.is_exported
                Signature      = $sym.signature
                DocComment     = $sym.doc_comment
                Complexity     = [int]$sym.complexity
                ParameterCount = [int]$sym.parameter_count
                ReceiverType   = $sym.receiver_type
                FunctionFlags  = $sym.function_flags
                VariableFlags  = $sym.variable_flags
                Callers        = $sym.callers
                Callees        = $sym.callees
                TypeHierarchy  = $sym.type_hierarchy
                ScopeChain     = $sym.scope_chain
                IncomingRefs   = [int]$sym.incoming_refs
                OutgoingRefs   = [int]$sym.outgoing_refs
                Annotations    = $sym.annotations
            }
        }
    }
}

function Get-LciFileOutline {
    <#
    .SYNOPSIS
        Browse all symbols in a file (outline / table-of-contents view).
    .DESCRIPTION
        Wraps 'lci browse --json'. Returns an LCI.FileOutline object
        whose Symbols property contains LCI.FileSymbol objects with
        navigation methods.
    .EXAMPLE
        Get-LciFileOutline "internal/server/server.go"
    .EXAMPLE
        Get-LciFileOutline "server.go" -Exported -Stats
    .EXAMPLE
        (Get-LciFileOutline "server.go").Symbols | Where-Object Type -eq 'function'
    #>
    [CmdletBinding()]
    [OutputType('LCI.FileOutline')]
    param(
        [Parameter(Mandatory, Position = 0, ValueFromPipelineByPropertyName)]
        [Alias('FilePath','_FilePath')]
        [string]$Path,

        [string]$Kind,
        [switch]$Exported,

        [ValidateSet('line','name','type','complexity','refs')]
        [string]$Sort = 'line',

        [switch]$Imports,
        [switch]$Stats
    )

    $args_ = @('browse', '--json', $Path, '--sort', $Sort)
    if ($Kind)     { $args_ += '--kind';     $args_ += $Kind }
    if ($Exported) { $args_ += '--exported' }
    if ($Imports)  { $args_ += '--imports' }
    if ($Stats)    { $args_ += '--stats' }

    $data = Invoke-Lci -Arguments $args_
    if (-not $data) { return }

    # Convert symbols
    $symbols = @()
    if ($data.symbols) {
        foreach ($sym in $data.symbols) {
            $symbols += New-LciObject -TypeName 'LCI.FileSymbol' -Properties @{
                Name           = $sym.name
                Type           = $sym.type
                File           = $sym.file
                Line           = $sym.line
                ObjectID       = $sym.object_id
                IsExported     = [bool]$sym.is_exported
                Signature      = $sym.signature
                Complexity     = [int]$sym.complexity
                ParameterCount = [int]$sym.parameter_count
                ReceiverType   = $sym.receiver_type
                IncomingRefs   = [int]$sym.incoming_refs
                OutgoingRefs   = [int]$sym.outgoing_refs
            }
        }
    }

    $outline = [PSCustomObject]@{
        PSTypeName = 'LCI.FileOutline'
        Path       = $data.file.path
        FileID     = $data.file.file_id
        Language   = $data.file.language
        Total      = [int]$data.total
        Imports    = $data.imports
        Stats      = $data.stats
        Symbols    = $symbols
    }

    # Add a shortcut to filter symbols
    $outline | Add-Member -MemberType ScriptMethod -Name 'Functions' -Force -Value {
        $this.Symbols | Where-Object { $_.Type -eq 'function' -or $_.Type -eq 'method' }
    }
    $outline | Add-Member -MemberType ScriptMethod -Name 'Types' -Force -Value {
        $this.Symbols | Where-Object { $_.Type -in @('struct','class','interface','type','enum') }
    }

    $outline
}

function Invoke-LciGitAnalysis {
    <#
    .SYNOPSIS
        Analyze git changes for duplicates and naming consistency.
    .DESCRIPTION
        Wraps 'lci git-analyze --json'. Returns an LCI.GitAnalysis object.
    .EXAMPLE
        Invoke-LciGitAnalysis
    .EXAMPLE
        Invoke-LciGitAnalysis -Scope wip
    .EXAMPLE
        Invoke-LciGitAnalysis -Scope range -Base main
    #>
    [CmdletBinding()]
    [OutputType('LCI.GitAnalysis')]
    param(
        [ValidateSet('staged','wip','commit','range')]
        [string]$Scope = 'staged',

        [string]$Base,
        [string]$Target,
        [string[]]$Focus,
        [double]$Threshold = 0.8,
        [int]$MaxFindings = 20
    )

    $args_ = @('git-analyze', '--json', '--scope', $Scope)
    if ($Base)                    { $args_ += '--base';         $args_ += $Base }
    if ($Target)                  { $args_ += '--target';       $args_ += $Target }
    if ($Focus)                   { foreach ($f in $Focus) { $args_ += '--focus'; $args_ += $f } }
    if ($Threshold -ne 0.8)       { $args_ += '--threshold';    $args_ += $Threshold }
    if ($MaxFindings -ne 20)      { $args_ += '--max-findings'; $args_ += $MaxFindings }

    $data = Invoke-Lci -Arguments $args_
    if (-not $data) { return }

    # Build duplicate objects with navigation
    $duplicates = @()
    if ($data.duplicates) {
        foreach ($dup in $data.duplicates) {
            $duplicates += [PSCustomObject]@{
                PSTypeName   = 'LCI.Duplicate'
                Type         = $dup.type
                Severity     = $dup.severity
                Similarity   = $dup.similarity
                NewCode      = $dup.new_code
                ExistingCode = $dup.existing_code
                Suggestion   = $dup.suggestion
            }
        }
    }

    $namingIssues = @()
    if ($data.naming_issues) {
        foreach ($issue in $data.naming_issues) {
            $namingIssues += [PSCustomObject]@{
                PSTypeName = 'LCI.NamingIssue'
                IssueType  = $issue.issue_type
                Severity   = $issue.severity
                Symbol     = $issue.new_symbol
                Issue      = $issue.issue
                Suggestion = $issue.suggestion
            }
        }
    }

    [PSCustomObject]@{
        PSTypeName      = 'LCI.GitAnalysis'
        Summary         = $data.summary
        Duplicates      = $duplicates
        NamingIssues    = $namingIssues
        Metadata        = $data.metadata
        FilesChanged    = [int]$data.summary.files_changed
        RiskScore       = [double]$data.summary.risk_score
        Recommendation  = $data.summary.top_recommendation
    }
}

function Start-LciServer {
    <#
    .SYNOPSIS Start the persistent index server.
    .DESCRIPTION
        Starts the lci index server. Note that most cmdlets auto-start
        the server if needed, so this is primarily useful for pre-warming.
    #>
    [CmdletBinding()]
    param([switch]$Daemon)

    $args_ = @('server')
    if ($Daemon) { $args_ += '--daemon' }

    $allArgs = @(Build-LciArgs) + $args_
    if ($Daemon) {
        Start-Process -FilePath $script:LciExe -ArgumentList $allArgs -WindowStyle Hidden
        Write-Host "LCI server started in background."
    } else {
        & $script:LciExe @allArgs
    }
}

function Stop-LciServer {
    <#
    .SYNOPSIS Shut down the persistent index server.
    #>
    [CmdletBinding()]
    param([switch]$Force)

    $args_ = @('shutdown')
    if ($Force) { $args_ += '--force' }

    try {
        Invoke-Lci -Arguments $args_ -RawText | Out-Null
        Write-Host "LCI server stopped."
    } catch {
        Write-Warning "Could not stop server: $_"
    }
}

# ---------------------------------------------------------------------------
# Default formatting
# ---------------------------------------------------------------------------

# Register default display properties for each type
Update-TypeData -TypeName 'LCI.SearchResult' -DefaultDisplayPropertySet @(
    'Path','Line','Match','BlockName','Score'
) -Force

Update-TypeData -TypeName 'LCI.GrepResult' -DefaultDisplayPropertySet @(
    'Path','Line','Column','Match'
) -Force

Update-TypeData -TypeName 'LCI.Definition' -DefaultDisplayPropertySet @(
    'Name','Type','FilePath','Line','Signature'
) -Force

Update-TypeData -TypeName 'LCI.Reference' -DefaultDisplayPropertySet @(
    'FilePath','Line','Context'
) -Force

Update-TypeData -TypeName 'LCI.Symbol' -DefaultDisplayPropertySet @(
    'Name','Type','File','Line','IsExported','Complexity','Signature'
) -Force

Update-TypeData -TypeName 'LCI.SymbolDetail' -DefaultDisplayPropertySet @(
    'Name','Type','File','Line','Signature','Complexity','Callers','Callees'
) -Force

Update-TypeData -TypeName 'LCI.FileSymbol' -DefaultDisplayPropertySet @(
    'Line','Type','Name','IsExported','Signature'
) -Force

Update-TypeData -TypeName 'LCI.TreeNode' -DefaultDisplayPropertySet @(
    'Name','FilePath','Line','ChildCount','EditRiskScore'
) -Force

# ---------------------------------------------------------------------------
# Tab completion
# ---------------------------------------------------------------------------

# Symbol kind completion
$SymbolKindCompleter = {
    param($commandName, $parameterName, $wordToComplete, $commandAst, $fakeBoundParameters)
    @('all','func','type','struct','interface','method','class','enum','variable','constant') |
        Where-Object { $_ -like "$wordToComplete*" } |
        ForEach-Object { [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_) }
}
Register-ArgumentCompleter -CommandName Get-LciSymbol -ParameterName Kind -ScriptBlock $SymbolKindCompleter

# Sort completion for symbols
$SymbolSortCompleter = {
    param($commandName, $parameterName, $wordToComplete, $commandAst, $fakeBoundParameters)
    @('name','complexity','refs','line','params') |
        Where-Object { $_ -like "$wordToComplete*" } |
        ForEach-Object { [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_) }
}
Register-ArgumentCompleter -CommandName Get-LciSymbol -ParameterName Sort -ScriptBlock $SymbolSortCompleter

# ---------------------------------------------------------------------------
# Aliases
# ---------------------------------------------------------------------------
New-Alias -Name lcis   -Value Search-LciCode     -Scope Global -Force
New-Alias -Name lcig   -Value Find-LciText        -Scope Global -Force
New-Alias -Name lcid   -Value Get-LciDefinition   -Scope Global -Force
New-Alias -Name lcir   -Value Get-LciReference     -Scope Global -Force
New-Alias -Name lcit   -Value Get-LciCallTree      -Scope Global -Force
New-Alias -Name lcisym -Value Get-LciSymbol        -Scope Global -Force
New-Alias -Name lcii   -Value Get-LciSymbolDetail  -Scope Global -Force
New-Alias -Name lcibr  -Value Get-LciFileOutline   -Scope Global -Force

# ---------------------------------------------------------------------------
# Module exports
# ---------------------------------------------------------------------------
Export-ModuleMember -Function @(
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
) -Alias @(
    'lcis', 'lcig', 'lcid', 'lcir', 'lcit', 'lcisym', 'lcii', 'lcibr'
)
