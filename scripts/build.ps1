param(
    [string]$Output = "yucca.exe"
)

$ErrorActionPreference = "Stop"

$version = "dev"

try {
    $exact = git describe --tags --exact-match 2>$null
    if ($LASTEXITCODE -eq 0 -and $exact) {
        $version = $exact.Trim()
    } else {
        $desc = git describe --tags --always --dirty 2>$null
        if ($LASTEXITCODE -eq 0 -and $desc) {
            $version = $desc.Trim()
        }
    }
} catch {
    # Keep default version when git is unavailable.
}

Write-Host "Building yucca version: $version"

go build `
  -ldflags "-X github.com/addidotlol/yucca/internal/meta.Version=$version" `
  -o $Output `
  ./cmd/yucca
