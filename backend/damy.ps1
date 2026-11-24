
$ErrorActionPreference = "Stop"

$secure = Read-Host "Enter the omicx_migrator password" -AsSecureString
$bstr = [IntPtr]::Zero
$password = $null
$encoded = $null

try {
    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    $password = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)

    if ([string]::IsNullOrWhiteSpace($password)) {
    throw "Password cannot be empty."
}
    $encoded = [Uri]::EscapeDataString($password)

    # Always use the .env beside this script.
    $envPath = Join-Path $PSScriptRoot ".env"

    if (-not (Test-Path $envPath)) {
        throw ".env was not found in the backend directory."
    }

    $lines = [IO.File]::ReadAllLines($envPath)

    $indexes = @()

    for ($i = 0; $i -lt $lines.Length; $i++) {
        if ($lines[$i] -match '^\s*MIGRATION_DATABASE_URL\s*=') {
            $indexes += $i
        }
    }

    if ($indexes.Count -ne 1) {
        throw "Expected exactly one MIGRATION_DATABASE_URL entry."
    }

    $newUrl = "postgres://omicx_migrator:$encoded@127.0.0.1:5433/omicx?sslmode=disable"

    $lines[$indexes[0]] = "MIGRATION_DATABASE_URL=$newUrl"

    [IO.File]::WriteAllLines(
        $envPath,
        $lines,
        [Text.UTF8Encoding]::new($false)
    )

    # Set the connection in the current PowerShell process.
    $env:MIGRATION_DATABASE_URL = $newUrl

    Write-Host "Migration connection updated successfully."
    Write-Host "Environment variable loaded successfully."
}
finally {
    if ($bstr -ne [IntPtr]::Zero) {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
    }

    $password = $null
    $encoded = $null
    $secure.Dispose()
}
