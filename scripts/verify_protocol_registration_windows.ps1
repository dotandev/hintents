$ErrorActionPreference = 'Stop'

$registryPath = 'Registry::HKEY_CURRENT_USER\Software\Classes\erst'
$commandPath = 'Registry::HKEY_CURRENT_USER\Software\Classes\erst\shell\open\command'

if (-not (Test-Path $registryPath)) {
    throw "Missing protocol registry key: $registryPath"
}

if (-not (Test-Path $commandPath)) {
    throw "Missing protocol open command key: $commandPath"
}

$rootKey = Get-Item $registryPath
$urlProtocol = $rootKey.GetValue('URL Protocol', $null)
if ($null -eq $urlProtocol) {
    throw 'Missing URL Protocol registry value'
}

$commandKey = Get-Item $commandPath
$command = $commandKey.GetValue('')
if ([string]::IsNullOrWhiteSpace($command)) {
    throw 'Missing default protocol open command'
}

if ($env:ERST_BINARY) {
    # Normalize paths to use forward slashes for cross-platform comparison
    $normalizedCommand = $command.Replace('\', '/')
    $normalizedBinary = $env:ERST_BINARY.Replace('\', '/')
    
    if ($normalizedCommand -notlike "*$normalizedBinary*") {
        throw "Protocol command does not reference expected binary: $normalizedBinary (got $normalizedCommand)"
    }
}

if ($command -notlike '*protocol-handler*') {
    throw 'Protocol command does not invoke protocol-handler'
}

Write-Host 'Windows protocol registration verified'