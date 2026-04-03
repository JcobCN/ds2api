param(
  [string]$Target = "vendor/kilo"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $Target)) {
  git clone --depth 1 https://github.com/Kilo-Org/kilocode.git $Target
  exit 0
}

git -C $Target fetch --depth 1 origin main
git -C $Target reset --hard origin/main
