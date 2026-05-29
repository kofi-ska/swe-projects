from __future__ import annotations

import asyncio
import pathlib
import random
import subprocess
import sys
import tempfile
from collections import Counter

from faker import Faker
import grpc


REPO_ROOT = pathlib.Path("/workspace/v1")
PROTO_DIR = REPO_ROOT / "contracts"


def compile_proto(out_dir: pathlib.Path) -> None:
    proto = PROTO_DIR / "decision.proto"
    subprocess.check_call(
        [
            sys.executable,
            "-m",
            "grpc_tools.protoc",
            f"-I{PROTO_DIR}",
            f"--python_out={out_dir}",
            f"--grpc_python_out={out_dir}",
            str(proto),
        ]
    )


def make_request(fake: Faker, idx: int, kind: str):
    import decision_pb2  # type: ignore

    if kind == "accept":
        quote_age = random.randint(1, 3)
        route_count = random.randint(2, 4)
    elif kind == "stale":
        quote_age = 99
        route_count = random.randint(2, 3)
    else:
        quote_age = random.randint(1, 3)
        route_count = 17

    route_candidates = [
        decision_pb2.RouteCandidate(
            route_id=f"{kind}-route-{idx}-{j}-{fake.uuid4()}",
            venue=fake.random_element(["pool", "amm", "aggregator", "venue"]),
            hop_count=1 + (j % 3),
        )
        for j in range(route_count)
    ]

    return decision_pb2.EvaluateSwapRequest(
        request_id=f"{kind}-request-{idx}-{fake.uuid4()}",
        dedupe_key=f"{kind}-dedupe-{idx}-{fake.uuid4()}",
        trace_id=f"{kind}-trace-{idx}-{fake.uuid4()}",
        model_version="v1",
        token_in=fake.random_element(["SOL", "USDC", "USDT", "BONK"]),
        token_out=fake.random_element(["SOL", "USDC", "USDT", "BONK"]),
        amount_in="100.0",
        route_id=route_candidates[0].route_id if route_candidates else "",
        slot=123456789 + idx,
        quote_age=quote_age,
        source_hashes=[fake.uuid4(), fake.uuid4()],
        route_candidates=route_candidates,
    )


async def call(stub, req):
    start = asyncio.get_running_loop().time()
    try:
        resp = await stub.EvaluateSwap(req, timeout=10.0)
        elapsed_ms = round((asyncio.get_running_loop().time() - start) * 1000, 2)
        return {
            "ok": True,
            "terminal_state": resp.terminal_state,
            "reason_code": resp.reason_code,
            "best_route_id": resp.best_route_id,
            "ev_lower_bound": resp.ev_lower_bound,
            "freshness_valid": resp.freshness_valid,
            "latency_ms": resp.compute_latency_ms,
            "round_trip_ms": elapsed_ms,
        }
    except grpc.aio.AioRpcError as exc:
        return {
            "ok": False,
            "code": exc.code().name,
            "details": exc.details(),
        }


async def main() -> int:
    fake = Faker()
    with tempfile.TemporaryDirectory(prefix="compute-smoke-") as tmp:
        out_dir = pathlib.Path(tmp)
        compile_proto(out_dir)
        sys.path.insert(0, str(out_dir))

        import decision_pb2_grpc  # type: ignore

        channel = grpc.aio.insecure_channel("compute:50051")
        await channel.channel_ready()
        stub = decision_pb2_grpc.ComputeServiceStub(channel)

        plan = ["accept"] * 70 + ["stale"] * 20 + ["oversized"] * 10
        random.shuffle(plan)
        results = await asyncio.gather(
            *[call(stub, make_request(fake, idx, kind)) for idx, kind in enumerate(plan)]
        )

        summary = Counter()
        samples = []
        for result in results:
            if result["ok"]:
                summary[f"ok:{result['terminal_state']}"] += 1
            else:
                summary[f"err:{result['code']}"] += 1
            if len(samples) < 8:
                samples.append(result)

        print("summary", dict(summary))
        print("samples")
        for sample in samples:
            print(sample)

        await channel.close()

    return 0


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
