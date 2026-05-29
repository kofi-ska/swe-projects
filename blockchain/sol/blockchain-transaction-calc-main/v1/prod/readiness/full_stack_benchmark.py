from __future__ import annotations

import pathlib
import subprocess
import sys


REPO_ROOT = pathlib.Path("/Users/settoramediku/Documents/Github/kofi-ska/swe-projects/blockchain/sol/blockchain-transaction-calc-main/v1")
COMPOSE_DIR = REPO_ROOT / "prod" / "deploy"


def main() -> int:
    cmd = [
        "docker",
        "compose",
        "exec",
        "-T",
        "orchestrator",
        "sh",
        "-lc",
        "exec sbt -Dsbt.color=false -batch 'runMain com.kofiska.solana.orchestrator.BenchmarkMain'",
    ]
    proc = subprocess.run(cmd, cwd=COMPOSE_DIR, text=True, capture_output=True)
    sys.stdout.write(proc.stdout)
    sys.stderr.write(proc.stderr)
    return proc.returncode


if __name__ == "__main__":
    raise SystemExit(main())
