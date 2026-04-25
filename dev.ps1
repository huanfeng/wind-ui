# Wind UI Developer Tool
# Usage:
#   .\dev.ps1           # list all demos
#   .\dev.ps1 1         # run demo #1
#   .\dev.ps1 1 3 5     # run demos #1, #3, #5 simultaneously
#   .\dev.ps1 all       # run all demos

$demos = @(
    @{ Name = "hello";         Desc = "Basic window, button click, XML layout" }
    @{ Name = "showcase";      Desc = "Widget showcase: Button, CheckBox, Switch, Radio, Progress" }
    @{ Name = "edittext";      Desc = "Native EditText, ScrollView, text input" }
    @{ Name = "phase3";        Desc = "Toolbar, TabLayout, ViewPager, RecyclerView" }
    @{ Name = "phase4";        Desc = "GridLayout, FlexLayout, Spinner, SeekBar, TreeView, SplitPane" }
    @{ Name = "clipman";       Desc = "Clipboard manager app (real-world example)" }
    @{ Name = "text_verify";   Desc = "Text measurement accuracy verification (offscreen PNG)" }
    @{ Name = "chinese_text";  Desc = "Chinese text rendering + mixed CJK (--window for DirectWrite)" }
    @{ Name = "p0_verify";     Desc = "Margin, PaintNode, keyboard, focus verification (--window)" }
)

function Show-Menu {
    Write-Host ""
    Write-Host "  Wind UI Demo Runner" -ForegroundColor Cyan
    Write-Host "  ===================" -ForegroundColor DarkCyan
    Write-Host ""
    for ($i = 0; $i -lt $demos.Count; $i++) {
        $num = ($i + 1).ToString().PadLeft(2)
        $name = $demos[$i].Name.PadRight(16)
        $desc = $demos[$i].Desc
        Write-Host "  $num) " -NoNewline -ForegroundColor Yellow
        Write-Host "$name" -NoNewline -ForegroundColor White
        Write-Host " $desc" -ForegroundColor DarkGray
    }
    Write-Host ""
    Write-Host "  Usage: " -NoNewline -ForegroundColor DarkGray
    Write-Host ".\dev.ps1 <num...>  " -NoNewline -ForegroundColor White
    Write-Host "e.g. .\dev.ps1 1 3 5" -ForegroundColor DarkGray
    Write-Host "         .\dev.ps1 all        run all window demos" -ForegroundColor DarkGray
    Write-Host ""
}

function Start-Demo {
    param([int]$Index)
    $demo = $demos[$Index]
    $name = $demo.Name
    $path = ".\examples\$name"

    # Offscreen demos: run inline; window demos: start as background process
    $offscreen = @("text_verify")
    if ($offscreen -contains $name) {
        Write-Host "  [$name] running offscreen..." -ForegroundColor Green
        go run $path
    } else {
        # For chinese_text and p0_verify, add --window flag
        $needsWindow = @("chinese_text", "p0_verify")
        if ($needsWindow -contains $name) {
            Write-Host "  [$name] launching with --window..." -ForegroundColor Green
            Start-Process -FilePath "go" -ArgumentList "run", $path, "--window" -WindowStyle Normal
        } else {
            Write-Host "  [$name] launching..." -ForegroundColor Green
            Start-Process -FilePath "go" -ArgumentList "run", $path -WindowStyle Normal
        }
    }
}

# No arguments: show menu
if ($args.Count -eq 0) {
    Show-Menu
    exit 0
}

# Handle "all" — run all window demos
if ($args[0] -eq "all") {
    Write-Host ""
    Write-Host "  Running all demos..." -ForegroundColor Cyan
    for ($i = 0; $i -lt $demos.Count; $i++) {
        Start-Demo -Index $i
    }
    Write-Host ""
    exit 0
}

# Parse numeric arguments
Write-Host ""
foreach ($arg in $args) {
    $num = 0
    if ([int]::TryParse($arg, [ref]$num)) {
        if ($num -ge 1 -and $num -le $demos.Count) {
            Start-Demo -Index ($num - 1)
        } else {
            Write-Host "  Invalid number: $arg (valid: 1-$($demos.Count))" -ForegroundColor Red
        }
    } else {
        Write-Host "  Invalid argument: $arg" -ForegroundColor Red
    }
}
Write-Host ""
