#!/usr/bin/env bash
# 从本机 QCCG (Keychain + ~/.qccg) 迁移账号到 qoder2api 文件存储
set -euo pipefail

QCCG_ACCOUNTS="${HOME}/.qccg/accounts"
DST_ACCOUNTS="${HOME}/.qoder2api/accounts"
DST_SECRETS="${HOME}/.qoder2api/secrets"

mkdir -p "$DST_ACCOUNTS" "$DST_SECRETS"

if [[ -d "$QCCG_ACCOUNTS" ]]; then
  cp -n "$QCCG_ACCOUNTS"/*.json "$DST_ACCOUNTS"/ 2>/dev/null || true
  echo "copied account json -> $DST_ACCOUNTS"
fi

if ! command -v security >/dev/null 2>&1; then
  echo "security CLI not found (non-macOS?). Skip keychain export."
  exit 0
fi

python3 - <<'PY'
import base64, os, pathlib, subprocess, sys

home = pathlib.Path.home()
accounts = home / ".qoder2api" / "accounts"
secrets = home / ".qoder2api" / "secrets"
secrets.mkdir(parents=True, exist_ok=True)

def decode(raw: str) -> str:
    raw = raw.strip()
    prefix = "go-keyring-base64:"
    if raw.startswith(prefix):
        enc = raw[len(prefix):]
        for dec in (base64.b64decode, base64.raw_b64decode if hasattr(base64, "raw_b64decode") else base64.b64decode):
            try:
                return base64.b64decode(enc).decode()
            except Exception:
                pass
        try:
            return base64.b64decode(enc + "==").decode()
        except Exception as e:
            print("decode fail:", e, file=sys.stderr)
            return raw
    return raw

for p in accounts.glob("*.json"):
    aid = p.stem
    try:
        raw = subprocess.check_output(
            ["security", "find-generic-password", "-s", "qccg", "-a", aid, "-w"],
            stderr=subprocess.DEVNULL,
            text=True,
        ).strip()
    except subprocess.CalledProcessError:
        print(f"skip {aid}: no keychain item")
        continue
    plain = decode(raw)
    out = secrets / f"{aid}.token"
    out.write_text(plain)
    out.chmod(0o600)
    print(f"migrated secret: {aid} ({len(plain)} bytes)")
PY

echo "done. restart: ./qoder2api --web-port=3588 --bridge-port=8963"
