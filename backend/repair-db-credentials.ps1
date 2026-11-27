
$ErrorActionPreference = "Stop"

$backend = "D:\my_company\mgx\omicx\backend"
Set-Location -LiteralPath $backend

$envPath = Join-Path $backend ".env"

if (-not (Test-Path $envPath)) {
    throw "backend/.env was not found."
}

# Read and validate the existing environment file.
$lines = [System.Collections.Generic.List[string]]::new()

foreach ($line in [IO.File]::ReadAllLines($envPath)) {
    [void]$lines.Add($line)
}

$dbIndexes = @()
$migrationIndexes = @()

for ($i = 0; $i -lt $lines.Count; $i++) {
    if ($lines[$i] -match '^\s*DATABASE_URL\s*=') {
        $dbIndexes += $i
    }

    if ($lines[$i] -match '^\s*MIGRATION_DATABASE_URL\s*=') {
        $migrationIndexes += $i
    }
}

if ($dbIndexes.Count -ne 1 -or $migrationIndexes.Count -ne 1) {
    throw "Expected exactly one DATABASE_URL and one MIGRATION_DATABASE_URL."
}

# Generate URL-safe, cryptographically random passwords.
function New-DatabasePassword {
    $bytes = New-Object byte[] 32
    $rng = [Security.Cryptography.RandomNumberGenerator]::Create()

    try {
        $rng.GetBytes($bytes)
    }
    finally {
        $rng.Dispose()
    }

    return [Convert]::ToBase64String($bytes).
        TrimEnd('=').
        Replace('+', '-').
        Replace('/', '_')
}

$appPassword = New-DatabasePassword
$migrationPassword = New-DatabasePassword

# Update both PostgreSQL roles through the local admin connection.
$sql = @"
ALTER ROLE omicx_app WITH PASSWORD '$appPassword';
ALTER ROLE omicx_migrator WITH PASSWORD '$migrationPassword';
"@

Write-Host "Updating PostgreSQL credentials..."

$sql | & docker exec -i omicx-postgres psql -U omicx -d omicx -v ON_ERROR_STOP=1

if ($LASTEXITCODE -ne 0) {
    throw "PostgreSQL rejected the credential update. .env was not changed."
}

# Update the correct backend/.env file.
$lines[$dbIndexes[0]] = "DATABASE_URL=postgres://omicx_app:$appPassword@127.0.0.1:5433/omicx?sslmode=disable"

$lines[$migrationIndexes[0]] = "MIGRATION_DATABASE_URL=postgres://omicx_migrator:$migrationPassword@127.0.0.1:5433/omicx?sslmode=disable"

[IO.File]::WriteAllLines(
    $envPath,
    $lines.ToArray(),
    [Text.UTF8Encoding]::new($false)
)

# Set both variables in this PowerShell process.
$env:DATABASE_URL = $lines[$dbIndexes[0]].Substring("DATABASE_URL=".Length)
$env:MIGRATION_DATABASE_URL = $lines[$migrationIndexes[0]].Substring("MIGRATION_DATABASE_URL=".Length)

# Remove secret variables from the PowerShell session.
$appPassword = $null
$migrationPassword = $null
$sql = $null

Write-Host ""
Write-Host "Both PostgreSQL passwords were updated."
Write-Host "backend/.env was synchronized."
Write-Host "Credentials are not printed."
Write-Host ""
Write-Host "Testing Tern connection..."

& tern status --migrations ./migrations

if ($LASTEXITCODE -ne 0) {
    throw "Tern connection verification failed. Credentials were updated; inspect the connection configuration."
}

Write-Host ""
Write-Host "Credential repair and Tern verification completed."
