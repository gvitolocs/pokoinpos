#!/usr/bin/env python3
import argparse
import json
import socket
import sys
import urllib.request
from datetime import datetime, timezone
from pathlib import Path


def utc_now():
    return datetime.now(timezone.utc)


def iso_now():
    return utc_now().replace(microsecond=0).isoformat().replace("+00:00", "Z")


def load_json(path, default):
    if not path.exists():
        return default
    with path.open("r", encoding="utf-8") as handle:
        return json.load(handle)


def save_json(path, payload):
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    with tmp.open("w", encoding="utf-8") as handle:
        json.dump(payload, handle, indent=2, sort_keys=True)
        handle.write("\n")
    tmp.replace(path)


def tcp_reachable(host, port, timeout):
    try:
        with socket.create_connection((host, port), timeout=timeout):
            return True, ""
    except OSError as error:
        return False, str(error)


def health_reachable(url, timeout):
    if not url:
        return True, "", ""
    try:
        with urllib.request.urlopen(url, timeout=timeout) as response:
            raw = response.read()
            if 200 <= response.status < 300:
                version = ""
                try:
                    payload = json.loads(raw.decode("utf-8"))
                    version = str(payload.get("version") or "")
                except Exception:
                    version = ""
                return True, "", version
            return False, f"http_status_{response.status}", ""
    except Exception as error:
        return False, str(error), ""


def probe(candidate, timeout):
    tcp_ok, tcp_error = tcp_reachable(candidate["host"], int(candidate["port"]), timeout)
    health_ok, health_error, version = health_reachable(candidate.get("opsHealthUrl", ""), timeout)
    live = bool(candidate.get("enabled", True)) and tcp_ok and health_ok
    return {
        "at": iso_now(),
        "id": candidate["id"],
        "observerId": "",
        "host": candidate["host"],
        "port": int(candidate["port"]),
        "live": live,
        "tcpOk": tcp_ok,
        "healthOk": health_ok,
        "version": version,
        "error": "" if live else (health_error or tcp_error),
    }


def update_summary(history):
    by_node = {}
    for observation in history.get("observations", []):
        node = by_node.setdefault(
            observation["id"],
            {
                "id": observation["id"],
                "host": observation["host"],
                "port": observation["port"],
                "totalChecks": 0,
                "liveChecks": 0,
                "downtimeChecks": 0,
                "lastSeenAt": "",
                "lastFailureAt": "",
                "currentStatus": "unknown",
            },
        )
        node["totalChecks"] += 1
        if observation["live"]:
            node["liveChecks"] += 1
            node["lastSeenAt"] = observation["at"]
            node["currentStatus"] = "live"
        else:
            node["downtimeChecks"] += 1
            node["lastFailureAt"] = observation["at"]
            node["currentStatus"] = "offline"
    for node in by_node.values():
        total = max(1, node["totalChecks"])
        node["uptimeRatio"] = round(node["liveChecks"] / total, 6)
    return sorted(by_node.values(), key=lambda item: item["id"])


def main():
    parser = argparse.ArgumentParser(description="Probe Pokoin bootstrap candidates.")
    parser.add_argument("--candidates", default="deploy/bootstrap/candidates.json")
    parser.add_argument("--history", default="deploy/bootstrap/uptime-history.json")
    parser.add_argument("--timeout", type=float, default=5.0)
    parser.add_argument("--observer-id", required=True, help="Stable ID of the peer/observer running this probe.")
    args = parser.parse_args()

    candidates_path = Path(args.candidates)
    history_path = Path(args.history)
    candidates = load_json(candidates_path, {"candidates": []})
    history = load_json(history_path, {"schemaVersion": 1, "observations": []})

    for candidate in candidates.get("candidates", []):
        if not candidate.get("enabled", True):
            continue
        observation = probe(candidate, args.timeout)
        observation["observerId"] = args.observer_id
        if observation["observerId"] == observation["id"]:
            # A node cannot certify its own uptime for bootstrap eligibility.
            continue
        history.setdefault("observations", []).append(observation)

    history["schemaVersion"] = 1
    history["updatedAt"] = iso_now()
    history["summary"] = update_summary(history)
    save_json(history_path, history)
    print(json.dumps({"updatedAt": history["updatedAt"], "summary": history["summary"]}, indent=2))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"bootstrap-monitor failed: {exc}", file=sys.stderr)
        sys.exit(1)
