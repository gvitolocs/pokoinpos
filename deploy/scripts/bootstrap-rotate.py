#!/usr/bin/env python3
import argparse
import json
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path


def parse_time(value):
    if not value:
        return None
    return datetime.fromisoformat(value.replace("Z", "+00:00"))


def iso_now():
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


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


def uptime_ratio(observations):
    if not observations:
        return 0.0
    live = sum(1 for item in observations if item.get("live"))
    return live / len(observations)


def observer_count(observations):
    observers = {
        item.get("observerId")
        for item in observations
        if item.get("observerId") and item.get("observerId") != item.get("id")
    }
    return len(observers)


def latest_version(observations):
    latest = None
    for item in observations:
        version = str(item.get("version") or "").strip()
        if not version:
            continue
        at = parse_time(item.get("at"))
        if at is None:
            continue
        if latest is None or at > latest[0]:
            latest = (at, version)
    return latest[1] if latest else ""


def candidate_version(candidate, observations):
    observed = latest_version(observations)
    if observed:
        return observed
    return str(candidate.get("version") or "").strip()


def age_days(candidate, now):
    first_seen = parse_time(candidate.get("firstSeenAt"))
    if first_seen is None:
        return 0
    return max(0, (now - first_seen).days)


def status_for_candidate(candidate, now, vetting_days, bootstrap_maturity_days, vetting_threshold, bootstrap_threshold, vetting_ratio, yearly_ratio):
    if candidate.get("grandfathered") and candidate.get("status") == "bootstrap":
        return "bootstrap"
    if candidate.get("status") == "rejected":
        return "rejected"
    age = age_days(candidate, now)
    vetting_ends = parse_time(candidate.get("vettingEndsAt")) or (parse_time(candidate.get("firstSeenAt")) + timedelta(days=vetting_days) if parse_time(candidate.get("firstSeenAt")) else None)
    if vetting_ends and now < vetting_ends:
        return "vetting"
    if vetting_ratio < vetting_threshold:
        return "failed_vetting"
    if age < bootstrap_maturity_days:
        return "peer"
    if yearly_ratio >= bootstrap_threshold:
        return "bootstrap"
    return "peer"


def main():
    parser = argparse.ArgumentParser(description="Generate Pokoin bootstrap peer manifest.")
    parser.add_argument("--candidates", default="deploy/bootstrap/candidates.json")
    parser.add_argument("--history", default="deploy/bootstrap/uptime-history.json")
    parser.add_argument("--output", default="deploy/bootstrap/bootstrap-peers.json")
    args = parser.parse_args()

    candidates = load_json(Path(args.candidates), {"policy": {}, "candidates": []})
    history = load_json(Path(args.history), {"observations": []})
    policy = candidates.get("policy", {})
    vetting_days = int(policy.get("vettingDays", 14))
    vetting_threshold = float(policy.get("vettingMinimumUptimeRatio", 0.95))
    bootstrap_maturity_days = int(policy.get("bootstrapMaturityDays", 365))
    window_days = int(policy.get("eligibilityWindowDays", 365))
    threshold = float(policy.get("minimumUptimeRatio", 0.94))
    min_external_observers = int(policy.get("minimumExternalObservers", 3))
    now = datetime.now(timezone.utc)
    cutoff = now - timedelta(days=window_days)

    observations_by_id = {}
    vetting_observations_by_id = {}
    for item in history.get("observations", []):
        at = parse_time(item.get("at"))
        if at is None or at < cutoff:
            continue
        observations_by_id.setdefault(item.get("id"), []).append(item)
    for candidate in candidates.get("candidates", []):
        start = parse_time(candidate.get("vettingStartedAt")) or parse_time(candidate.get("firstSeenAt"))
        end = parse_time(candidate.get("vettingEndsAt"))
        if start is None:
            continue
        if end is None:
            end = start + timedelta(days=vetting_days)
        for item in history.get("observations", []):
            at = parse_time(item.get("at"))
            if at is None or at < start or at > end:
                continue
            vetting_observations_by_id.setdefault(candidate.get("id"), []).append(item)

    peers = []
    candidate_statuses = []
    for candidate in candidates.get("candidates", []):
        if not candidate.get("enabled", True):
            continue
        vetting_observations = vetting_observations_by_id.get(candidate["id"], [])
        observations = observations_by_id.get(candidate["id"], [])
        ratio = uptime_ratio(observations)
        external_observers = observer_count(observations)
        vetting_ratio = uptime_ratio(vetting_observations)
        if not vetting_observations and age_days(candidate, now) < vetting_days:
            vetting_ratio = 1.0
        status = status_for_candidate(candidate, now, vetting_days, bootstrap_maturity_days, vetting_threshold, threshold, vetting_ratio, ratio)
        if status == "bootstrap" and not candidate.get("grandfathered") and external_observers < min_external_observers:
            status = "peer"
        candidate_statuses.append(
            {
                "id": candidate["id"],
                "label": candidate.get("label", candidate["id"]),
                "host": candidate["host"],
                "port": int(candidate["port"]),
                "version": candidate_version(candidate, observations),
                "status": status,
                "ageDays": age_days(candidate, now),
                "externalObservers": external_observers,
                "vettingUptimeRatio": round(vetting_ratio, 6),
                "uptimeRatio365d": round(ratio, 6),
                "grandfathered": bool(candidate.get("grandfathered", False)),
            }
        )
        eligible = status == "bootstrap"
        if eligible:
            peers.append(
                {
                    "id": candidate["id"],
                    "label": candidate.get("label", candidate["id"]),
                    "host": candidate["host"],
                    "port": int(candidate["port"]),
                    "version": candidate_version(candidate, observations),
                    "status": status,
                    "ageDays": age_days(candidate, now),
                    "externalObservers": external_observers,
                    "vettingUptimeRatio": round(vetting_ratio, 6),
                    "uptimeRatio365d": round(ratio if observations else 1.0, 6),
                    "grandfathered": bool(candidate.get("grandfathered", False)),
                }
            )

    fallback = [f"{candidate['host']}:{int(candidate['port'])}" for candidate in candidates.get("candidates", []) if candidate.get("fallback", False)]
    fallback_candidates = [candidate for candidate in candidates.get("candidates", []) if candidate.get("fallback", False)]
    default_join = fallback_candidates[0] if fallback_candidates else None
    manifest = {
        "schemaVersion": 1,
        "generatedAt": iso_now(),
        "validForHours": 24,
        "network": {
            "name": "PokoinPoS",
            "bootstrapManifestUrl": "https://pokoin.com/bootstrap-peers.json",
            "bootstrapRefreshIntervalHours": 24,
        },
        "evm": {
            "chainId": 26062026,
            "networkId": "26062026",
        },
        "bootstrap": {
            "fallbackPeers": fallback,
            "defaultJoinPeer": {
                "host": default_join["host"],
                "port": int(default_join["port"]),
            } if default_join else None,
        },
        "policy": {
            "vettingDays": vetting_days,
            "vettingMinimumUptimeRatio": vetting_threshold,
            "bootstrapMaturityDays": bootstrap_maturity_days,
            "eligibilityWindowDays": window_days,
            "minimumUptimeRatio": threshold,
            "minimumExternalObservers": min_external_observers,
        },
        "peers": peers,
        "candidates": candidate_statuses,
        "fallbackPeers": fallback,
    }
    save_json(Path(args.output), manifest)
    print(json.dumps(manifest, indent=2))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"bootstrap-rotate failed: {exc}", file=sys.stderr)
        sys.exit(1)
