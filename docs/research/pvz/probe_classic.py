"""Run selected REA MCP probes in one owned static-analysis session.

No target process is executed. Set TMPDIR to the current worktree root and use
existing GHIDRA_INSTALL_DIR/JAVA_HOME. All retained Evidence goes to --output.
"""
import argparse
import json
import os
from pathlib import Path
import subprocess


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("target", type=Path)
    parser.add_argument("jobs", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    environment = os.environ.copy()
    environment["TMPDIR"] = str(Path.cwd())
    stderr = (args.output / "rea-session.log").open("w")
    process = subprocess.Popen(["rea", "--mcp"], env=environment, stdin=subprocess.PIPE,
                               stdout=subprocess.PIPE, stderr=stderr, text=True, bufsize=1)
    sequence = 0

    def call(method, params):
        nonlocal sequence
        sequence += 1
        request = {"jsonrpc": "2.0", "id": sequence, "method": method, "params": params}
        process.stdin.write(json.dumps(request) + "\n")
        process.stdin.flush()
        for line in process.stdout:
            response = json.loads(line)
            if response.get("id") != sequence:
                continue
            if "error" in response:
                raise RuntimeError(response["error"])
            return response["result"]
        raise RuntimeError("REA session ended before response")

    def tool(name, arguments):
        result = call("tools/call", {"name": name, "arguments": arguments})
        if result.get("isError"):
            raise RuntimeError(result)
        structured = result.get("structuredContent")
        if structured is not None:
            return structured
        text = next((x["text"] for x in result.get("content", []) if x.get("type") == "text"), "{}")
        return json.loads(text)

    try:
        call("initialize", {"protocolVersion": "2025-06-18", "capabilities": {},
                            "clientInfo": {"name": "wire-board-pvz-evidence", "version": "1"}})
        process.stdin.write(json.dumps({"jsonrpc": "2.0", "method": "notifications/initialized"}) + "\n")
        process.stdin.flush()
        inventory = call("tools/list", {})
        available = {x["name"] for x in inventory["tools"]}
        tool("open_binary", {"path": str(args.target.resolve()), "provider_id": "ghidra"})
        for job in json.loads(args.jobs.read_text()):
            if job["tool"] not in available:
                raise ValueError(f"REA tool unavailable: {job['tool']}")
            try:
                result = tool(job["tool"], job["arguments"])
            except RuntimeError as error:
                (args.output / (job["name"] + ".unknown.json")).write_text(json.dumps({
                    "status": "unknown", "job": job, "reason": str(error),
                }, indent=2) + "\n")
                print(job["name"], "unknown (retained provider error)", flush=True)
                continue
            if not result.get("evidence_id"):
                raise ValueError(f"No canonical Evidence for {job['name']}")
            (args.output / (job["name"] + ".json")).write_text(json.dumps(result, indent=2) + "\n")
            print(job["name"], result["evidence_id"], flush=True)
    finally:
        try:
            tool("close_binary", {})
        except (RuntimeError, BrokenPipeError):
            pass
        process.stdin.close()
        process.wait(timeout=30)
        stderr.close()


if __name__ == "__main__":
    main()
